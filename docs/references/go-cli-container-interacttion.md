# **Go言語を用いたコンテナ内プロセスの対話型CLIツールの開発：アーキテクチャと実装パターンの詳解**

現代のソフトウェア開発において、DockerやPodmanに代表されるコンテナ技術は不可欠な基盤となっている。これらの技術はアプリケーションのポータビリティを向上させた一方で、隔離された環境内で実行されるプロセスとの対話には、洗練されたインターフェース設計が求められるようになった。開発者がホスト側のターミナルからコンテナ内のシェルを直接操作しているかのような「透過的な」体験を提供するためには、Unix系オペレーティングシステムの伝統的な擬似端末（Pseudo-Terminal, PTY）の仕組みと、Go言語が提供する並行処理モデルを深く理解し、それらを統合する必要がある。

本報告書では、Go言語を用いてコンテナ内プロセスと通信する対話型CLIツールを構築するための、基礎的なアーキテクチャから実装の詳細、シグナル伝搬、ウィンドウリサイズへの対応、そしてクロスプラットフォームでの考慮事項に至るまでを、技術的な知見に基づき網羅的に解説する。

## **擬似端末（PTY）の基礎理論とアーキテクチャ**

対話型CLIの実装における核心は、擬似端末（PTY）の概念にある。通常のプログラム実行（例えば ls コマンドの実行）では、標準入力、標準出力、標準エラー出力の三つのストリームを単にパイプで繋ぐだけで十分である。しかし、vim や htop、あるいは対話型シェル（bash や zsh）のように、画面描画を制御したり、特定のキー入力（Ctrl+Cなど）に反応したりするアプリケーションの場合、実行環境が「ターミナル（TTY）」であることを認識している必要がある 1。

PTYは、カーネル内に存在するソフトウェア的なデバイスペアであり、「マスタ（Master）」と「スレーブ（Slave）」の二つの端点で構成される 3。マスタ側はターミナルエミュレータやCLIツール本体が保持し、スレーブ側はシェルなどの子プロセスに接続される 3。カーネルはこのペアの間でデータを転送する際、「ライン規律（Line Discipline）」と呼ばれる処理層を介在させる 3。この層が、入力された文字の表示（エコーバック）や、特定の制御文字によるシグナルの生成を司っている。

| 構成要素 | 役割 | 動作の概要 |
| :---- | :---- | :---- |
| PTYマスタ | CLIツールが制御する端点 | ユーザのキー入力を書き込み、プロセスからの出力を読み取る 4。 |
| PTYスレーブ | 子プロセスがターミナルとして認識する端点 | 標準入出力として子プロセスに割り当てられ、プロセスはこの端点を本物の物理端末と見なす 3。 |
| ライン規律 | データの加工・変換 | 入力データのバッファリング、エコーバック、Ctrl+C等によるシグナル生成を担当する 3。 |
| カーネル | データの仲介 | マスタとスレーブ間の物理的なデータ転送を管理し、シグナルを適切なプロセスグループに送信する 3。 |

コンテナ内でプロセスを実行する場合、CLIツールはホスト側のPTYマスタから読み取ったデータを、コンテナエンジンのAPI経由でコンテナ内のプロセスの標準入力へ送り、逆にコンテナ内プロセスからの出力をホスト側のターミナルに描画するという「リレー」の役割を果たすことになる 7。

## **Go言語におけるI/Oリレーの実装：os/execとio.Copy**

Go言語で外部プロセスを起動する際の標準的な手段は os/exec パッケージである。しかし、単純に exec.Command を使用して起動しただけでは、そのプロセスはPTYを持たない 1。対話型ツールを実現するためには、プロセスの起動時に明示的にPTYを割り当てる必要がある。これを行うための最も一般的なライブラリが github.com/creack/pty である 4。

creack/pty ライブラリの pty.Start(cmd) 関数は、新しいPTYマスタとスレーブのペアを作成し、作成したスレーブ側を exec.Cmd の標準入力、標準出力、標準エラー出力に自動的に割り当てた上でプロセスを開始する 4。戻り値として得られるのはPTYマスタを示す \*os.File であり、これを通じて双方向の通信を行うことができる 4。

このPTYマスタとホスト側の標準入出力（os.Stdin, os.Stdout）を接続するために、Goの強力な抽象化である io.Copy が利用される 6。io.Copy は、リーダー（Reader）からライター（Writer）へデータがなくなるまでコピーを続ける関数であるが、対話型通信では「ユーザ入力の転送」と「プロセス出力の表示」を同時に行う必要があるため、これらを並行して実行しなければならない 12。

実装の一般的なパターンは以下の通りである。

1. pty.Start でプロセスとPTYを開始する 4。  
2. ゴルーチンを生成し、io.Copy(ptyMaster, os.Stdin) を実行して、キーボード入力をプロセスへ送る 6。  
3. メインのスレッド（または別のゴルーチン）で io.Copy(os.Stdout, ptyMaster) を実行し、プロセスからの出力をターミナルに表示する 6。

この際、io.Copy はブロッキング操作であるため、プロセスが終了したときに適切にこれらのゴルーチンを終了させることが、メモリリークやリソースの浪費を防ぐための重要な課題となる 12。

## **ターミナル制御の極意：Rawモードと標準入出力の管理**

デフォルト状態のターミナルは「カノニカルモード（Canonical Mode）」または「クックドモード（Cooked Mode）」と呼ばれ、オペレーティングシステムがユーザの入力を加工してからプログラムに渡す 16。具体的には、ユーザが「Enter」キーを押すまで入力データはバッファリングされ、バックスペースによる修正などがOSレベルで処理される 17。また、入力された文字が即座に画面に表示される「エコー」もこのモードの機能である 17。

対話型シェルやエディタをコンテナ内で動かす場合、このホスト側の加工処理が邪魔になる 16。例えば、Ctrl+Cを押したときにホスト側のOSがそれを解釈してCLIツール自体を終了させてしまうと、コンテナ内のプロセスにシグナルを送ることができない 16。この問題を解決するために、クライアント側のターミナルを「Rawモード」に設定する必要がある 16。

Rawモードでは、OSによる入力の加工やエコーが全て無効化され、全てのキー入力がそのままのバイト列としてアプリケーション（今回の場合はGoで作られたCLIツール）に渡される 16。これにより、矢印キーの特殊なエスケープシーケンスや、Ctrl+Cなどの制御コードをアプリケーションが検知し、それをコンテナ内プロセスへリレーすることが可能になる 16。

Go言語では、golang.org/x/term パッケージを使用してこの設定を行うのがベストプラクティスである 16。term.MakeRaw 関数は、指定したファイル記述子（通常は os.Stdin）のターミナル設定を変更し、変更前の状態（State）を戻り値として返す 16。プログラムの終了時には、この保存された状態を使用して必ずターミナルを元のモードに「復元（Restore）」しなければならない 16。復元を怠ると、プログラム終了後のシェルでも文字が表示されなかったり、改行が正しく動作しなかったりといった不具合が発生し、ユーザの利便性を著しく損なうことになる 16。

| フラグ（termios） | 効果（Rawモード時） | 理由 |
| :---- | :---- | :---- |
| ECHO | 無効化 | 入力文字のエコーはコンテナ内のプロセスが行うため、ホスト側で行うと二重に表示されてしまう 3。 |
| ICANON | 無効化 | 1文字入力ごとに即座にプロセスへ送信するため、行単位のバッファリングを解除する 17。 |
| ISIG | 無効化 | Ctrl+CやCtrl+Zによるホスト側でのシグナル生成を抑制し、アプリケーションが直接受信できるようにする 16。 |
| IEXTEN | 無効化 | 拡張入力処理を無効化し、純粋なデータとして全ての文字を扱う 17。 |

## **シグナルハンドリング：Ctrl+C、Ctrl+D、そしてプロセスのライフサイクル**

対話型アプリケーションにおけるシグナルの扱いは非常に繊細である。Rawモードを導入することで、ホストOSによる自動的なシグナル生成は抑制されるが、今度はアプリケーション自身がユーザの意図を汲み取り、それを適切にコンテナ内へ伝えなければならない。

### **Ctrl+C（SIGINT）と Ctrl+D（EOF）の区別**

多くのユーザにとって、Ctrl+Cは「現在の操作の中断」を意味し、Ctrl+Dは「入力の終了（EOF）」を意味する 16。Rawモード下では、Ctrl+Cを検知してもアプリケーションは即座に終了せず、その「バイトデータ（0x03）」をPTYマスタへ書き込む 16。すると、PTYスレーブ側のライン規律がこれを解釈し、コンテナ内で実行されているプロセスに対して SIGINT シグナルを送信する 3。これが、シェルで実行中のコマンドを中断できる仕組みの正体である。

一方、Ctrl+Dはシグナルではなく、入力ストリームの終了（EOF）を示す 16。Goの実装において、os.Stdin から 0x04 を読み取った場合、あるいは io.Copy が io.EOF を返した場合、入力側のストリームを閉じる処理を行う必要がある 6。

### **SIGINTとSIGTERMのトラップと転送**

CLIツールそのものが外部から SIGINT や SIGTERM を受け取った場合（例えば、ツールがバックグラウンドで動いている際に kill コマンドを受けた場合）、ツールは自身だけが終了するのではなく、管理下にあるコンテナ内プロセスも適切に終了させる責任がある 21。

Goでは os/signal パッケージの signal.Notify を使用してこれらのシグナルをチャネルで受信する 20。シグナルを受信した際のクリーンアップ手順は以下のようになる。

1. コンテナエンジン（Docker API等）を通じて、コンテナ内のプロセスにシグナルを転送する 22。  
2. 一定の猶予（Grace Period）を持ってプロセスの終了を待つ 22。  
3. ターミナルの状態をRestoreし、プログラムを終了する 16。

特に、コンテナ内の「PID 1（初期プロセス）」問題には注意が必要である 25。Dockerコンテナでは、最初に実行されたコマンドがPID 1として扱われるが、Linuxのカーネル仕様により、PID 1のプロセスは明示的なハンドラがない限りシグナルを無視する性質がある 25。これを防ぐためには、コンテナ起動時に docker run \--init フラグを使用するか、シェルスクリプト経由で実行する場合は exec コマンドを使用してプロセスを置き換える等の工夫が求められる 22。

## **ウィンドウリサイズの動的同期：SIGWINCHとPTYの調整**

ターミナルベースのアプリケーション、特に vim のように画面全体を使用する「TUI（Terminal User Interface）」アプリにとって、ターミナルの行数と列数の情報は極めて重要である。ユーザがターミナルウィンドウの端をドラッグしてサイズを変更したとき、ホスト側のターミナルドライバはフォアグラウンドのプロセスに対して SIGWINCH（Signal Window Change）を送信する 27。

コンテナ内のプロセスにこの変更を伝えるためには、CLIツールがこの SIGWINCH を捕捉し、新しいサイズを取得した上で、PTYマスタに対してサイズ変更の ioctl を発行しなければならない 27。

### **実装の手順**

1. **シグナルの監視**: os/signal を使用して syscall.SIGWINCH を受信するチャネルを設定する 29。  
2. **現在サイズの取得**: ホスト側のターミナル（os.Stdout など）に対して TIOCGWINSZ を発行し、現在の行数と列数を取得する 27。  
3. **PTYへの適用**: 取得したサイズ情報を pty.Setsize などの関数を用いてPTYマスタに適用する 4。

Go

// SIGWINCHのハンドリング例  
sigChan := make(chan os.Signal, 1)  
signal.Notify(sigChan, syscall.SIGWINCH)  
go func() {  
    for range sigChan {  
        // ホスト端末のサイズを取得し、PTYに継承させる  
        if err := pty.InheritSize(os.Stdin, ptyMaster); err\!= nil {  
            log.Printf("サイズ変更に失敗: %v", err)  
        }  
    }  
}()

このリレーが正常に行われると、カーネルはコンテナ内のPTYスレーブに対しても SIGWINCH を送信し、それを受けたコンテナ内プロセス（例えば bash）が環境変数 LINES や COLUMNS を更新、あるいはアプリケーションが再描画を行うことで、表示の崩れを防ぐことができる 27。

## **クロスプラットフォームの課題：Unix PTYとWindows ConPTY**

CLIツールの開発において、Linux/macOSとWindowsの両方をサポートすることは大きな挑戦である。Unix系システムでは /dev/ptmx を中心とした標準的なPTYの仕組みが確立されているが、Windowsには長年これに相当する仕組みが存在しなかった。

### **Windows ConPTY の登場**

Windows 10 バージョン 1809 以降、Microsoftは「Windows Pseudo Console (ConPTY)」APIを導入した 11。これにより、WindowsでもUnixライクなPTY操作が可能になったが、その実装モデルはUnixとは大きく異なる。ConPTYでは、マスタとスレーブという単純なペアではなく、ホストアプリケーションが作成した「入力用」と「出力用」の二つのパイプを ConPTY インスタンスに渡すことで通信を確立する 33。

| 特徴 | Unix系 (Linux/macOS) | Windows (ConPTY) |
| :---- | :---- | :---- |
| デバイスモデル | /dev/ptmx デバイスファイル 5 | CreatePseudoConsole API 呼び出し 32 |
| サイズ変更 | ioctl(fd, TIOCSWINSZ,...) 27 | ResizePseudoConsole(handle, size) 32 |
| シグナル | SIGINT, SIGWINCH 等 27 | コンソール入力イベント（または win32-input-mode） 36 |
| ライブラリ対応 | creack/pty が標準的 4 | aymanbagabas/go-pty 等の抽象化層が必要 11 |

### **win32-input-mode の複雑さ**

Windows ターミナルで対話型アプリを動かす際、特に高度な制御を行うために win32-input-mode が有効化されることがある 36。このモードでは、単なるASCII文字の代わりに、キーの押し下げ（Key Down）や離し（Key Up）、複数の修飾キーの状態を含む詳細なエスケープシーケンスが送られてくる 36。Goでツールを作成する場合、これらのシーケンスをパースして適切な文字データに変換するロジックが必要になる 36。また、Windowsでは SIGWINCH という概念自体が存在しないため、ウィンドウサイズの変更を検知するには別の方法（例えば ReadConsoleInput で WINDOW\_BUFFER\_SIZE\_EVENT を待つ、あるいはポーリングする）が必要となる 34。

クロスプラットフォーム対応を容易にするためには、OSごとの差異を抽象化してくれるライブラリを選択することが賢明である。aymanbagabas/go-pty は Unix PTY と Windows ConPTY の両方をサポートしており、共通のインターフェースで操作できるため、コードの保守性が向上する 11。

## **堅牢なクリーンアップとリソース管理：ゴルーチンリークの防止**

対話型CLIツールの運用において、最も発生しやすいバグの一つがゴルーチンリークである 13。特に io.Copy を使用してリレーを行っている場合、その性質上リークが発生しやすい 14。

### **io.Copy のブロッキング問題**

io.Copy は、読み込み元が EOF を返すか、書き込み先でエラーが発生するまで戻ってこない 12。コンテナ内のプロセスが終了しても、ホスト側の標準入力（os.Stdin）を担当している io.Copy は、ユーザが何かキーを入力するまで Read でブロックし続けてしまう 14。これは、プロセスが終了しているのにいつまでも背後でゴルーチンが生き残り、リソースを消費し続ける原因となる 14。

### **対策としての非ブロッキングI/OとContext**

この問題を回避し、堅牢な終了処理を実現するためには以下のテクニックを組み合わせる。

1. **プロセスの終了検知と強制クローズ**: cmd.Wait() でプロセスの終了を検知したら、即座に PTY マスタのファイル記述子を閉じる 1。これにより、他のゴルーチンで実行されている io.Copy は書き込みエラーを検知して終了することができる 12。  
2. **Context によるキャンセル**: 長時間実行される操作には必ず context.Context を紐付け、アプリケーション終了時に全ての関連処理が停止するように設計する 41。  
3. **非ブロッキングモードの設定**: 必要に応じて syscall.SetNonblock を使用し、I/O操作にタイムアウトを設定できるようにする 6。これにより、いつまでも戻ってこない Read 操作を防ぐことが可能になる。

また、デバッグ段階では runtime.NumGoroutine() を定期的に出力して監視したり、uber-go/goleak のようなライブラリをテストスイートに組み込んだりすることで、開発の早い段階でリークを発見できる 39。

### **終了コードの返却と復元**

ユーザにとって、CLIツールがコンテナ内プロセスの終了コードを正しく返すことは、スクリプトによる自動化などにおいて不可欠である。Goでは、cmd.Wait() の結果を解析して終了コードを取得する 46。

Go

err := cmd.Wait()  
if err\!= nil {  
    if exiterr, ok := err.(\*exec.ExitError); ok {  
        // コンテナ内プロセスの終了ステータスを取得  
        exitCode := exiterr.ExitCode()  
        os.Exit(exitCode) \[46, 47\]  
    }  
}

この処理の前に、term.Restore を呼び出してターミナルを正常な状態に戻すことを忘れてはならない 16。正常な終了シーケンスは、

1. プロセス終了待機  
2. ターミナル復元  
3. リソース解放（PTYクローズ、ゴルーチン停止確認）  
4. 終了コードを伴う os.Exit  
   という順序で行われるべきである。

## **実践的な実装例とライブラリの活用**

具体的な開発において、車輪の再発明を避けるために既存の高度なライブラリを活用することは非常に効果的である。

### **1\. aymanbagabas/go-pty**

このライブラリは、Unix PTY と Windows ConPTY の差異を隠蔽するための強力なツールである 11。特に Windows サポートが充実しており、os/exec だけでは困難な ConPTY の属性設定を自動で行ってくれる 11。対話型ツールにおいてクロスプラットフォーム展開を想定しているなら、第一の選択肢となる。

### **2\. golang.org/x/term**

標準ライブラリの拡張パッケージであり、Rawモードへの切り替え（MakeRaw）やターミナルサイズの取得（GetSize）を提供している 4。外部依存を最小限に抑えつつ、OS標準の機能を確実に利用したい場合に適している。

### **3\. Docker Go SDK (moby/moby)**

コンテナエンジンと直接通信する場合、外部コマンドの docker exec を叩くのではなく、SDKを直接利用する手法もある 7。SDKの ContainerExecCreate および ContainerExecAttach を使用すると、コンテナ内のプロセスと直接ストリーム（net.Conn）を確立できる 7。このストリームを PTY ハンドリングロジックに流し込むことで、より統合されたツールを作成できる。

| 要件 | 推奨されるアプローチ | 理由 |
| :---- | :---- | :---- |
| 最小限の依存関係 | os/exec \+ creack/pty | Linux/Unix環境では非常に軽量で安定している 4。 |
| Windows完全対応 | aymanbagabas/go-pty | ConPTY の複雑なAPIをGoの exec.Cmd ライクなインターフェースにラップしている 11。 |
| 高度なUI (TUI) | charmbracelet/bubbletea | PTY ハンドリングと高度な画面描画フレームワークを組み合わせるのに適している。 |
| コンテナエンジン直販 | Docker SDK | 外部バイナリへの依存を排除し、ネットワーク経由でのコンテナ操作も容易にする 7。 |

## **結論：堅牢な対話型ツールを目指して**

Go言語を使用してコンテナ内プロセスと対話するCLIツールを構築することは、単なるプログラミング作業を超え、OSの低レイヤ機能とコンテナ技術を橋渡しするシステムエンジニアリングの課題である。PTYのアーキテクチャ、Rawモードの制御、シグナルの適切な伝搬、そしてウィンドウリサイズの同期といった要素が一つでも欠ければ、ユーザ体験は損なわれ、実用的なツールとはなり得ない。

特に現代の開発環境では、異なるOS間での互換性と、並行処理に伴うリソース管理（ゴルーチンリークの防止）が重要な差別化要因となる。io.Copy を単純に使うのではなく、その裏にあるブロッキングの仕組みを理解し、プロセスのライフサイクルに合わせて適切に制御することが、プロフェッショナルなCLIツール作成の極意であると言える 14。

本報告書で詳説したパターンとベストプラクティスを遵守することで、DockerやPodmanといったコンテナ環境の隔離性を維持しつつ、あたかもその境界が存在しないかのような、スムーズで強力な対話型操作環境をユーザに提供することが可能となる。

#### **引用文献**

1. How to execute interactive CLI command in golang? \- Stack Overflow, 1月 31, 2026にアクセス、 [https://stackoverflow.com/questions/54418628/how-to-execute-interactive-cli-command-in-golang](https://stackoverflow.com/questions/54418628/how-to-execute-interactive-cli-command-in-golang)  
2. How to access the pseudo terminal's stderr? · Issue \#147 · creack/pty \- GitHub, 1月 31, 2026にアクセス、 [https://github.com/creack/pty/issues/147](https://github.com/creack/pty/issues/147)  
3. Linux terminals, tty, pty and shell \- part 2 \- DEV Community, 1月 31, 2026にアクセス、 [https://dev.to/napicella/linux-terminals-tty-pty-and-shell-part-2-2cb2](https://dev.to/napicella/linux-terminals-tty-pty-and-shell-part-2-2cb2)  
4. pty package \- github.com/creack/pty \- Go Packages, 1月 31, 2026にアクセス、 [https://pkg.go.dev/github.com/creack/pty](https://pkg.go.dev/github.com/creack/pty)  
5. pty.Start seems to close the terminal too early · Issue \#127 · creack/pty \- GitHub, 1月 31, 2026にアクセス、 [https://github.com/creack/pty/issues/127](https://github.com/creack/pty/issues/127)  
6. creack/pty: PTY interface for Go \- GitHub, 1月 31, 2026にアクセス、 [https://github.com/creack/pty](https://github.com/creack/pty)  
7. Examples using the Docker Engine SDKs and Docker API, 1月 31, 2026にアクセス、 [https://docs.docker.com/reference/api/engine/sdk/examples/](https://docs.docker.com/reference/api/engine/sdk/examples/)  
8. Interactive Docker exec with docker-py \- Stack Overflow, 1月 31, 2026にアクセス、 [https://stackoverflow.com/questions/78154889/interactive-docker-exec-with-docker-py](https://stackoverflow.com/questions/78154889/interactive-docker-exec-with-docker-py)  
9. Podman Exec: A Beginner's Guide | Better Stack Community, 1月 31, 2026にアクセス、 [https://betterstack.com/community/guides/scaling-docker/podman-exec/](https://betterstack.com/community/guides/scaling-docker/podman-exec/)  
10. Advanced command execution in Go with os/exec, 1月 31, 2026にアクセス、 [https://blog.kowalczyk.info/article/wOYk/advanced-command-execution-in-go-with-osexec.html](https://blog.kowalczyk.info/article/wOYk/advanced-command-execution-in-go-with-osexec.html)  
11. aymanbagabas/go-pty: Cross platform Go Pty interface \- GitHub, 1月 31, 2026にアクセス、 [https://github.com/aymanbagabas/go-pty](https://github.com/aymanbagabas/go-pty)  
12. go \- io.Copy in goroutine to prevent blocking \- Stack Overflow, 1月 31, 2026にアクセス、 [https://stackoverflow.com/questions/62522276/io-copy-in-goroutine-to-prevent-blocking](https://stackoverflow.com/questions/62522276/io-copy-in-goroutine-to-prevent-blocking)  
13. Understanding and Preventing Goroutine Leaks in Go | by SONU RAJ \- Medium, 1月 31, 2026にアクセス、 [https://medium.com/@srajsonu/understanding-and-preventing-goroutine-leaks-in-go-623cac542954](https://medium.com/@srajsonu/understanding-and-preventing-goroutine-leaks-in-go-623cac542954)  
14. io: Copy is easy to misuse and leak goroutines blocked on reads · Issue \#58628 · golang/go, 1月 31, 2026にアクセス、 [https://github.com/golang/go/issues/58628](https://github.com/golang/go/issues/58628)  
15. Non-blocking I/O in Go \- Medium, 1月 31, 2026にアクセス、 [https://medium.com/@cpuguy83/non-blocking-i-o-in-go-bc4651e3ac8d](https://medium.com/@cpuguy83/non-blocking-i-o-in-go-bc4651e3ac8d)  
16. Go Tidbit: Putting The Terminal Into Raw Mode · hjr265.me, 1月 31, 2026にアクセス、 [https://hjr265.me/blog/go-tidbit-putting-the-terminal-into-raw-mode/](https://hjr265.me/blog/go-tidbit-putting-the-terminal-into-raw-mode/)  
17. Building a Terminal Raw Mode Input Reader in Go \- Mariano Zunino, 1月 31, 2026にアクセス、 [https://mzunino.com.uy/til/2025/03/building-a-terminal-raw-mode-input-reader-in-go/](https://mzunino.com.uy/til/2025/03/building-a-terminal-raw-mode-input-reader-in-go/)  
18. 2\. Entering raw mode | Build Your Own Text Editor, 1月 31, 2026にアクセス、 [https://viewsourcecode.org/snaptoken/kilo/02.enteringRawMode.html](https://viewsourcecode.org/snaptoken/kilo/02.enteringRawMode.html)  
19. Writing an interactive CLI menu in Golang \- Medium, 1月 31, 2026にアクセス、 [https://medium.com/@nexidian/writing-an-interactive-cli-menu-in-golang-d6438b175fb6](https://medium.com/@nexidian/writing-an-interactive-cli-menu-in-golang-d6438b175fb6)  
20. Read user input until he press ctrl+c? \- golang \- Reddit, 1月 31, 2026にアクセス、 [https://www.reddit.com/r/golang/comments/4hktbe/read\_user\_input\_until\_he\_press\_ctrlc/](https://www.reddit.com/r/golang/comments/4hktbe/read_user_input_until_he_press_ctrlc/)  
21. Handling CTRL-C (interrupt signal) in Golang Programs \- Nathan LeClaire, 1月 31, 2026にアクセス、 [https://nathanleclaire.com/blog/2014/08/24/handling-ctrl-c-interrupt-signal-in-golang-programs/](https://nathanleclaire.com/blog/2014/08/24/handling-ctrl-c-interrupt-signal-in-golang-programs/)  
22. How to handle kill signal in go (inside a docker container) \- awesomeprogrammer.com, 1月 31, 2026にアクセス、 [https://awesomeprogrammer.com/blog/2020/01/04/how-to-handle-kill-signal-in-go-inside-a-docker-container/](https://awesomeprogrammer.com/blog/2020/01/04/how-to-handle-kill-signal-in-go-inside-a-docker-container/)  
23. Unexpected interaction between Go and Docker Compose | by Aleksa Novcic \- Medium, 1月 31, 2026にアクセス、 [https://medium.com/@aleksa-novcic/unexpected-interaction-between-go-and-docker-compose-510e6791ac17](https://medium.com/@aleksa-novcic/unexpected-interaction-between-go-and-docker-compose-510e6791ac17)  
24. Docker stack or service kill to send signal to remote containers \- Feature Requests, 1月 31, 2026にアクセス、 [https://forums.docker.com/t/docker-stack-or-service-kill-to-send-signal-to-remote-containers/43481](https://forums.docker.com/t/docker-stack-or-service-kill-to-send-signal-to-remote-containers/43481)  
25. Best practices for propagating signals on Docker \- Kaggle, 1月 31, 2026にアクセス、 [https://www.kaggle.com/code/residentmario/best-practices-for-propagating-signals-on-docker](https://www.kaggle.com/code/residentmario/best-practices-for-propagating-signals-on-docker)  
26. Sending signals to Golang application in Docker \- Stack Overflow, 1月 31, 2026にアクセス、 [https://stackoverflow.com/questions/33379567/sending-signals-to-golang-application-in-docker](https://stackoverflow.com/questions/33379567/sending-signals-to-golang-application-in-docker)  
27. Playing with SIGWINCH \- R. Koucha, 1月 31, 2026にアクセス、 [http://www.rkoucha.fr/tech\_corner/sigwinch.html](http://www.rkoucha.fr/tech_corner/sigwinch.html)  
28. How do terminal size changes get sent to command line applications though ssh or telnet?, 1月 31, 2026にアクセス、 [https://stackoverflow.com/questions/19157202/how-do-terminal-size-changes-get-sent-to-command-line-applications-though-ssh-or](https://stackoverflow.com/questions/19157202/how-do-terminal-size-changes-get-sent-to-command-line-applications-though-ssh-or)  
29. Go Tidbit: Detect When the Terminal Is Resized \- hjr265.me, 1月 31, 2026にアクセス、 [https://hjr265.me/blog/go-tidbit-detect-when-the-terminal-is-resized/](https://hjr265.me/blog/go-tidbit-detect-when-the-terminal-is-resized/)  
30. Responsive Terminal Applications in Golang \- Reevik, 1月 31, 2026にアクセス、 [https://reevik.net/Responsive-Terminal-Applications/](https://reevik.net/Responsive-Terminal-Applications/)  
31. Issue 41494: Adds window resizing support to Lib/pty.py \[ SIGWINCH \] \- Python tracker, 1月 31, 2026にアクセス、 [https://bugs.python.org/issue41494](https://bugs.python.org/issue41494)  
32. ResizePseudoConsole function \- Windows Console \- Microsoft Learn, 1月 31, 2026にアクセス、 [https://learn.microsoft.com/en-us/windows/console/resizepseudoconsole](https://learn.microsoft.com/en-us/windows/console/resizepseudoconsole)  
33. Creating a Pseudoconsole session \- Windows Console \- Microsoft Learn, 1月 31, 2026にアクセス、 [https://learn.microsoft.com/en-us/windows/console/creating-a-pseudoconsole-session](https://learn.microsoft.com/en-us/windows/console/creating-a-pseudoconsole-session)  
34. conpty package \- github.com/charmbracelet/x/conpty \- Go Packages, 1月 31, 2026にアクセス、 [https://pkg.go.dev/github.com/charmbracelet/x/conpty](https://pkg.go.dev/github.com/charmbracelet/x/conpty)  
35. How Golang implement stdin/stdout/stderr \- Stack Overflow, 1月 31, 2026にアクセス、 [https://stackoverflow.com/questions/38773244/how-golang-implement-stdin-stdout-stderr](https://stackoverflow.com/questions/38773244/how-golang-implement-stdin-stdout-stderr)  
36. Taming Windows Terminal's win32-input-mode in Go ConPTY ..., 1月 31, 2026にアクセス、 [https://dev.to/andylbrummer/taming-windows-terminals-win32-input-mode-in-go-conpty-applications-7gg](https://dev.to/andylbrummer/taming-windows-terminals-win32-input-mode-in-go-conpty-applications-7gg)  
37. SIGWINCH equivalent on Windows? \- Stack Overflow, 1月 31, 2026にアクセス、 [https://stackoverflow.com/questions/10856926/sigwinch-equivalent-on-windows](https://stackoverflow.com/questions/10856926/sigwinch-equivalent-on-windows)  
38. ptyx package \- github.com/KennethanCeyer/ptyx \- Go Packages, 1月 31, 2026にアクセス、 [https://pkg.go.dev/github.com/KennethanCeyer/ptyx](https://pkg.go.dev/github.com/KennethanCeyer/ptyx)  
39. Detecting goroutine leaks with synctest/pprof \- Anton Zhiyanov, 1月 31, 2026にアクセス、 [https://antonz.org/detecting-goroutine-leaks/](https://antonz.org/detecting-goroutine-leaks/)  
40. How To Find and Fix Goroutine Leaks in Go \- DZone, 1月 31, 2026にアクセス、 [https://dzone.com/articles/how-to-find-and-fix-goroutine-leaks-in-go?fromrel=true](https://dzone.com/articles/how-to-find-and-fix-goroutine-leaks-in-go?fromrel=true)  
41. Go Concurrency Mastery: Preventing Goroutine Leaks with Context, Timeout & Cancellation Best Practices \- DEV Community, 1月 31, 2026にアクセス、 [https://dev.to/serifcolakel/go-concurrency-mastery-preventing-goroutine-leaks-with-context-timeout-cancellation-best-1lg0](https://dev.to/serifcolakel/go-concurrency-mastery-preventing-goroutine-leaks-with-context-timeout-cancellation-best-1lg0)  
42. Golang io.Copy blocks in internal ReadFrom \- Stack Overflow, 1月 31, 2026にアクセス、 [https://stackoverflow.com/questions/58477293/golang-io-copy-blocks-in-internal-readfrom](https://stackoverflow.com/questions/58477293/golang-io-copy-blocks-in-internal-readfrom)  
43. Goroutine Leaks in Go \- Medium, 1月 31, 2026にアクセス、 [https://medium.com/@AlexanderObregon/goroutine-leaks-in-go-ece4824df9a1](https://medium.com/@AlexanderObregon/goroutine-leaks-in-go-ece4824df9a1)  
44. syscall.SetNonblock stopped working in Go 1.9.3 \- Google Groups, 1月 31, 2026にアクセス、 [https://groups.google.com/g/golang-nuts/c/uk\_HozBGg\_Y](https://groups.google.com/g/golang-nuts/c/uk_HozBGg_Y)  
45. Detecting goroutine leaks with synctest/pprof : r/golang \- Reddit, 1月 31, 2026にアクセス、 [https://www.reddit.com/r/golang/comments/1pqhgnz/detecting\_goroutine\_leaks\_with\_synctestpprof/](https://www.reddit.com/r/golang/comments/1pqhgnz/detecting_goroutine_leaks_with_synctestpprof/)  
46. Get exit code \- Go \- Stack Overflow, 1月 31, 2026にアクセス、 [https://stackoverflow.com/questions/10385551/get-exit-code-go](https://stackoverflow.com/questions/10385551/get-exit-code-go)  
47. Better way to get the exit status of an os/exec command? \- Google Groups, 1月 31, 2026にアクセス、 [https://groups.google.com/g/golang-nuts/c/sFmzNL-0zq4](https://groups.google.com/g/golang-nuts/c/sFmzNL-0zq4)  
48. docker container exec \- Docker Docs, 1月 31, 2026にアクセス、 [https://docs.docker.com/reference/cli/docker/container/exec/](https://docs.docker.com/reference/cli/docker/container/exec/)
