#!/usr/bin/env python3

import argparse
import subprocess
import os
import sys

ALLOWED_EXTENSIONS = {'.py', '.go', '.ts', '.cs', '.html', '.css', '.js', '.tmpl', '.txt', '.md'}

def get_new_files():
    result = subprocess.run(['git', 'status', '--porcelain'], capture_output=True, text=True)
    new_files = []
    for line in result.stdout.strip().split('\n'):
        if not line:
            continue
        status = line[:2]
        filepath = line[3:].strip()
        if status.startswith('??'):
            ext = os.path.splitext(filepath)[1].lower()
            if ext in ALLOWED_EXTENSIONS:
                new_files.append(filepath)
    return new_files

def get_modified_files():
    result = subprocess.run(['git', 'status', '--porcelain'], capture_output=True, text=True)
    modified_files = []
    for line in result.stdout.strip().split('\n'):
        if not line:
            continue
        status = line[:2]
        filepath = line[3:].strip()
        if status[0] in ('M', 'A'):
            modified_files.append(filepath)
    return modified_files

def stage_and_commit(step, spec_file):
    new_files = get_new_files()
    modified_files = get_modified_files()
    
    files_to_add = new_files + modified_files
    
    if not files_to_add:
        print(f"No changes detected for step {step}; skipping commit")
        return

    subprocess.run(['git', 'add'] + files_to_add, check=True)
    
    commit_msg = f"implemented step {step} in {spec_file}"
    subprocess.run(['git', 'commit', '-m', commit_msg], check=True)

def run_opencode(spec_file, step, model, repo_root):
    prompt = f"load {spec_file} and implement step {step}. do not implement other steps. run 'make quality' to verify. make sure to complete all other acceptance test steps as specified in the spec."
    # opencode treats the first positional argument as a working directory.
    # Keep the spec file in the prompt instead of passing it positionally.
    cmd = ['opencode', '--prompt', prompt, '--model', model]
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

    print(f"Implementing spec: {spec_file} with {args.steps} steps")

    for step in range(1, args.steps + 1):
        print(f"\n=== Running step {step}/{args.steps} ===")
        
        ret = run_opencode(spec_file, step, args.model, repo_root)
        
        if ret != 0:
            print(f"Error: opencode exited with code {ret}; stopping")
            sys.exit(ret)
        
        print(f"Committing step {step}...")
        stage_and_commit(step, spec_file)
        print(f"Step {step} complete")

    print("\n=== All steps complete ===")

if __name__ == '__main__':
    main()