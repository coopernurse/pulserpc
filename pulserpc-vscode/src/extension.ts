import * as vscode from 'vscode';

export function activate(context: vscode.ExtensionContext) {
    // Extension is activated for .pulse files
    // The syntax highlighting is provided by the TextMate grammar
    console.log('PulseRPC Language Support extension is now active!');
}

export function deactivate() {}
