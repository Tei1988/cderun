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
    target_pattern = re.compile(rf'^##\s+{re.escape(task_id)}\b', re.IGNORECASE)
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

def remove_details_only(task_id):
    lines = load_todo_file()

    new_lines = []
    in_section = False
    target_pattern = re.compile(rf'^##\s+{re.escape(task_id)}\b', re.IGNORECASE)
    any_task_pattern = re.compile(r'^##\s+T\d+\b')
    removed = False

    # Detect the blank line before the header if any, to remove excess space
    skip_prev_hr = False
    for line in lines:
        if target_pattern.match(line):
            in_section = True
            removed = True
            # Check if previous lines were "---" or blank lines, and try to remove them
            # to prevent piling up of separators.
            # But let's keep it simple: we just don't add the line.
            continue
        elif in_section and any_task_pattern.match(line):
            in_section = False
            new_lines.append(line)
        elif in_section:
            continue
        else:
            new_lines.append(line)

    if removed:
        # Clean up any trailing/duplicate "---" and empty lines
        content = "".join(new_lines)
        content = re.sub(r'\n---\n\s*---\n', '\n---\n', content)
        content = re.sub(r'\n\s*---\s*\n\s*$', '\n', content)
        save_todo_file([content])
        print(f"Successfully removed detailed instructions section for {task_id}.")
    else:
        print(f"Detailed section for {task_id} not found or already deleted.")

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

    save_todo_file(lines)
    print(f"Successfully marked {task_id} as DONE in summary table.")

    # 2. Remove detailed section
    remove_details_only(task_id)

def add_task(task_id, title, task_type, priority, size, spec_change, details_content):
    lines = load_todo_file()

    # Check if task already exists
    tasks = parse_tasks(lines)
    if any(t['id'].lower() == task_id.lower() for t in tasks):
        print(f"Error: Task {task_id} already exists.", file=sys.stderr)
        sys.exit(1)

    # Create the new table row
    new_row = f"| {task_id} | {title} | {task_type} | {priority} | {size} | {spec_change} | - |\n"

    # Find the insertion point for the table row (right after the last row in the table)
    last_table_idx = -1
    row_pattern = re.compile(r'^\|\s*(T\d+)\s*\|')
    for idx, line in enumerate(lines):
        if row_pattern.match(line):
            last_table_idx = idx

    if last_table_idx == -1:
        print("Error: Could not locate the summary table in todo.md.", file=sys.stderr)
        sys.exit(1)

    # Insert the new row
    lines.insert(last_table_idx + 1, new_row)

    # Append the detailed instructions at the bottom of the file
    if details_content:
        # Normalize newline at the end of todo.md
        joined = "".join(lines)
        if not joined.endswith("\n"):
            joined += "\n"

        # Append formatted detail section
        formatted_details = f"\n---\n\n## {task_id}: {title}\n\n{details_content}\n"
        joined += formatted_details
        lines = [joined]

    save_todo_file(lines)
    print(f"Successfully added task {task_id} and its details to todo.md.")

def main():
    parser = argparse.ArgumentParser(description="Manage agent tasks in cderun todo.md.")
    subparsers = parser.add_subparsers(dest="command", help="Command to run")

    subparsers.add_parser("list", help="List all tasks and their status")

    show_parser = subparsers.add_parser("show", help="Show details of a specific task")
    show_parser.add_argument("task_id", help="The Task ID (e.g., T58)")

    done_parser = subparsers.add_parser("done", help="Mark a task as DONE and remove its details")
    done_parser.add_argument("task_id", help="The Task ID (e.g., T58)")

    delete_details_parser = subparsers.add_parser("delete-details", help="Remove detailed instructions of a task without marking it DONE")
    delete_details_parser.add_argument("task_id", help="The Task ID (e.g., T58)")

    add_parser = subparsers.add_parser("add", help="Add a new task with optional details")
    add_parser.add_argument("task_id", help="The Task ID (e.g., T99)")
    add_parser.add_argument("title", help="Title of the task")
    add_parser.add_argument("type", help="Type of the task (e.g., 機能, バグ, 改善, リファクタ, 調査)")
    add_parser.add_argument("priority", help="Priority of the task (e.g., 高, 中, 低)")
    add_parser.add_argument("size", help="Scale/Size of the task (e.g., 大, 中, 小)")
    add_parser.add_argument("--spec-change", default="-", help="Whether spec changes are involved (e.g. あり or -)")
    add_parser.add_argument("details", nargs="?", default="", help="Detailed instructions to be added at the bottom of todo.md")

    args = parser.parse_args()

    if args.command == "list":
        list_tasks()
    elif args.command == "show":
        show_task(args.task_id)
    elif args.command == "done":
        mark_done(args.task_id)
    elif args.command == "delete-details":
        remove_details_only(args.task_id)
    elif args.command == "add":
        add_task(args.task_id, args.title, args.type, args.priority, args.size, args.spec_change, args.details)
    else:
        parser.print_help()

if __name__ == "__main__":
    main()
