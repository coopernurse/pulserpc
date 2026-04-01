#!/usr/bin/env python3

import argparse
import json
import subprocess
import os
import sys

ALLOWED_EXTENSIONS = {'.py', '.go', '.ts', '.cs', '.html', '.css', '.js', '.tmpl', '.txt', '.md'}
STATE_FILE = os.path.expanduser('~/.implement-ai.json')

def load_state():
    if not os.path.exists(STATE_FILE):
        return {}
    with open(STATE_FILE, 'r', encoding='utf-8') as f:
        data = json.load(f)
    if not isinstance(data, dict):
        raise ValueError(f"state file must contain a JSON object: {STATE_FILE}")
    return data

def save_state(state):
    state_dir = os.path.dirname(STATE_FILE)
    if state_dir:
        os.makedirs(state_dir, exist_ok=True)
    with open(STATE_FILE, 'w', encoding='utf-8') as f:
        json.dump(state, f, indent=2, sort_keys=True)
        f.write('\n')

def update_last_completed_step(spec_file, step):
    state = load_state()
    state[spec_file] = step
    save_state(state)

def get_start_step(spec_file):
    state = load_state()
    last_step = state.get(spec_file, 0)
    if not isinstance(last_step, int) or last_step < 0:
        raise ValueError(f"invalid stored step for {spec_file}: {last_step}")
    return last_step + 1

def get_changed_files():
    """Return all new/modified files with allowed extensions using unambiguous git commands."""
    cmds = [
        # unstaged working-tree modifications to tracked files
        ['git', 'diff', '--name-only'],
        # staged modifications (index vs HEAD)
        ['git', 'diff', '--cached', '--name-only'],
        # untracked files not covered by .gitignore
        ['git', 'ls-files', '--others', '--exclude-standard'],
    ]
    seen = set()
    files = []
    for cmd in cmds:
        result = subprocess.run(cmd, capture_output=True, text=True, check=True)
        for line in result.stdout.splitlines():
            path = line.strip()
            if path and path not in seen:
                ext = os.path.splitext(path)[1].lower()
                if ext in ALLOWED_EXTENSIONS:
                    seen.add(path)
                    files.append(path)
    return files

def stage_and_commit(step, spec_path_for_commit):
    files_to_add = get_changed_files()

    if not files_to_add:
        print(f"No changes detected for step {step}; skipping commit")
        return

    subprocess.run(['git', 'add'] + files_to_add, check=True)
    
    commit_msg = f"implemented step {step} in {spec_path_for_commit}"
    subprocess.run(['git', 'commit', '-m', commit_msg], check=True)

def run_opencode(spec_file, step, model, repo_root):
    prompt = f"load {spec_file} and implement step {step}. do not implement other steps. run 'make quality' to verify. make sure to complete all other acceptance test steps as specified in the spec."
    # opencode treats the first positional argument as a working directory.
    # Keep the spec file in the prompt instead of passing it positionally.
    cmd = ['opencode', 'run', prompt, '--model', model]
    result = subprocess.run(cmd, cwd=repo_root)
    return result.returncode

def main():
    parser = argparse.ArgumentParser(description='Automate opencode implementation of spec steps')
    parser.add_argument('spec_file', help='Path to the markdown spec file')
    parser.add_argument('steps', type=int, help='Number of steps to implement')
    parser.add_argument('-m', '--model', default='opencode/minimax-m2.5-free',
                        help='model to use in the format of provider/model (default: opencode/minimax-m2.5-free)')
    args = parser.parse_args()

    spec_file = os.path.abspath(args.spec_file)
    if not os.path.isfile(spec_file):
        parser.error(f"spec file does not exist: {spec_file}")

    repo_root = subprocess.run(
        ['git', 'rev-parse', '--show-toplevel'],
        capture_output=True,
        text=True,
        check=True,
    ).stdout.strip()
    spec_file_rel = os.path.relpath(spec_file, repo_root)

    start_step = get_start_step(spec_file)

    if start_step > args.steps:
        print(f"All steps already complete for {spec_file} (last completed: {start_step - 1})")
        return

    print(f"Implementing spec: {spec_file} with {args.steps} steps (starting at step {start_step})")

    for step in range(start_step, args.steps + 1):
        print(f"\n=== Running step {step}/{args.steps} ===")
        
        ret = run_opencode(spec_file, step, args.model, repo_root)
        
        if ret != 0:
            print(f"Error: opencode exited with code {ret}; stopping")
            sys.exit(ret)
        
        print(f"Committing step {step}...")
        stage_and_commit(step, spec_file_rel)
        update_last_completed_step(spec_file, step)
        print(f"Step {step} complete")

    print("\n=== All steps complete ===")

if __name__ == '__main__':
    main()