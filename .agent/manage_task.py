#!/usr/bin/env python3
import sys
import os
import re
import argparse

TODO_FILE = os.path.join(os.path.dirname(__file__), 'todo.md')

def load_todo_file():
    if not os.path.exists(TODO_FILE):
        print(f"Error: {TODO_FILE} not found.", file=sys.stderr)
        sys.exit(1)
    with open(TODO_FILE, 'r', encoding='utf-8') as f:
        return f.readlines()

def save_todo_file(lines):
    with open(TODO_FILE, 'w', encoding='utf-8') as f:
        f.writelines(lines)

def parse_tasks(lines):
    tasks = []
    # Regular expression to match the task table rows: | ID | Title | Type | Priority | Size | Spec | Status |
    # e.g., | T01 | TTY ... | 調査 | 高 | ? | - | - |
    row_pattern = re.compile(r'^\|\s*(T\d+)\s*\|([^|]+)\|([^|]+)\|([^|]+)\|([^|]+)\|([^|]+)\|([^|]+)\|')
    for line in lines:
        m = row_pattern.match(line)
        if m:
            task_id = m.group(1).strip()
            title = m.group(2).strip()
            status = m.group(7).strip()
            tasks.append({
                'id': task_id,
                'title': title,
                'status': status
            })
    return tasks

def list_tasks():
    lines = load_todo_file()
    tasks = parse_tasks(lines)
    print(f"{'ID':<6} | {'Status':<10} | {'Title'}")
    print("-" * 60)
    for t in tasks:
        print(f"{t['id']:<6} | {t['status']:<10} | {t['title']}")

def show_task(task_id):
    lines = load_todo_file()
    tasks = parse_tasks(lines)
    task = next((t for t in tasks if t['id'].lower() == task_id.lower()), None)
    if not task:
        print(f"Error: Task {task_id} not found in the summary table.", file=sys.stderr)
        sys.exit(1)

    print(f"Task ID: {task['id']}")
    print(f"Title:   {task['title']}")
    print(f"Status:  {task['status']}")
    print("\nDetailed Instructions:")
    print("-" * 40)

    # Extract detailed description
    target_pattern = re.compile(rf'^##\s+{task_id}\b', re.IGNORECASE)
    any_task_pattern = re.compile(r'^##\s+T\d+\b')
    in_section = False
    details = []

    for line in lines:
        if target_pattern.match(line):
            in_section = True
            details.append(line)
        elif in_section and any_task_pattern.match(line):
            break
        elif in_section:
            details.append(line)

    if details:
        print("".join(details))
    else:
        print("(No detailed instructions found in todo.md)")

def mark_done(task_id):
    lines = load_todo_file()

    # 1. Update the table row
    updated_table = False
    for i, line in enumerate(lines):
        if line.strip().startswith('|') and f'| {task_id} |' in line:
            parts = line.split('|')
            if len(parts) >= 8:
                parts[-2] = ' DONE '
                lines[i] = '|'.join(parts)
                updated_table = True
                break

    if not updated_table:
        print(f"Error: Task {task_id} not found in the summary table.", file=sys.stderr)
        sys.exit(1)

    # 2. Remove detailed section
    new_lines = []
    in_section = False
    target_pattern = re.compile(rf'^##\s+{task_id}\b', re.IGNORECASE)
    any_task_pattern = re.compile(r'^##\s+T\d+\b')

    for line in lines:
        if target_pattern.match(line):
            in_section = True
            continue
        elif in_section and any_task_pattern.match(line):
            in_section = False
            new_lines.append(line)
        elif in_section:
            continue
        else:
            new_lines.append(line)

    save_todo_file(new_lines)
    print(f"Successfully marked {task_id} as DONE and removed its detailed instructions.")

def main():
    parser = argparse.ArgumentParser(description="Manage agent tasks in cderun todo.md.")
    subparsers = parser.add_subparsers(dest="command", help="Command to run")

    subparsers.add_parser("list", help="List all tasks and their status")

    show_parser = subparsers.add_parser("show", help="Show details of a specific task")
    show_parser.add_argument("task_id", help="The Task ID (e.g., T58)")

    done_parser = subparsers.add_parser("done", help="Mark a task as DONE and remove its details")
    done_parser.add_argument("task_id", help="The Task ID (e.g., T58)")

    args = parser.parse_args()

    if args.command == "list":
        list_tasks()
    elif args.command == "show":
        show_task(args.task_id)
    elif args.command == "done":
        mark_done(args.task_id)
    else:
        parser.print_help()

if __name__ == "__main__":
    main()
