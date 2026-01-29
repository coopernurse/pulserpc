"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.activate = activate;
exports.deactivate = deactivate;
function activate(context) {
    // Extension is activated for .pulse files
    // The syntax highlighting is provided by the TextMate grammar
    console.log('PulseRPC Language Support extension is now active!');
}
function deactivate() { }
//# sourceMappingURL=extension.js.map