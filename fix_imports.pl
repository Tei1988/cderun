use strict;
use warnings;

undef $/;
my $content = <>;

if ($content =~ /import\s*\((.*?)\)/s) {
    my $original_import_block = $1;
    my @lines = split(/\n/, $original_import_block);

    my @stdlib;
    my @local;
    my @thirdparty;

    foreach my $line (@lines) {
        next if $line =~ /^\s*$/;
        if ($line =~ /"(?:archive|bufio|bytes|context|crypto|database|debug|embed|encoding|errors|expvar|flag|fmt|go|hash|html|image|index|io|log|math|mime|net|os|path|plugin|reflect|regexp|runtime|sort|strconv|strings|sync|syscall|testing|text|time|unicode|unsafe)/ || $line =~ /^\s*[a-z]+\s+"(?:archive|...)/) {
            # This is a bit simplistic for stdlib but should work for common ones.
            # Better check: if it doesn't contain a dot in the first path component, it's likely stdlib.
            if ($line =~ /^\s*(?:[a-z0-9_]+\s+)?"([^.]+?)(?:\/.*)?"/ ) {
                 push @stdlib, $line;
            } elsif ($line =~ /"(?:github\.com|gopkg\.in|golang\.org|dario\.cat)/) {
                 push @thirdparty, $line;
            } elsif ($line =~ /"cderun\//) {
                 push @local, $line;
            } else {
                 # Fallback
                 push @thirdparty, $line;
            }
        } elsif ($line =~ /"cderun\//) {
            push @local, $line;
        } else {
            push @thirdparty, $line;
        }
    }

    my $new_import_block = "";
    $new_import_block .= join("\n", sort @stdlib) . "\n\n" if @stdlib;
    $new_import_block .= join("\n", sort @local) . "\n\n" if @local;
    $new_import_block .= join("\n", sort @thirdparty) . "\n" if @thirdparty;
    $new_import_block =~ s/\n\n$/\n/; # remove trailing double newline

    $content =~ s/import\s*\(.*?\)/import (\n$new_import_block\n)/s;
}

print $content;
