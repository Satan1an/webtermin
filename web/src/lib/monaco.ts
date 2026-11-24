// Serve Monaco from our own bundle. @monaco-editor/react otherwise fetches it
// from cdn.jsdelivr.net, which the panel's CSP (script-src 'self') blocks and
// which an offline LAN box can't reach anyway. Imported only by the lazy
// Files/Stacks pages, so the editor stays out of the main chunk.
//
// Editor core with all editor features, tokenizers for the languages
// FilesPage's guessLanguage maps, and the JSON language service — through the
// tree-shakeable entry points monaco-editor has offered since 0.56. The full
// "monaco-editor" entry would also pull the TS/CSS/HTML services and their
// workers (~9 MB in the binary).
import { loader } from "@monaco-editor/react";
import * as monaco from "monaco-editor/editor";
import "monaco-editor/features/register.all";
import "monaco-editor/languages/features/json/register";
import "monaco-editor/languages/definitions/dockerfile/register";
import "monaco-editor/languages/definitions/go/register";
import "monaco-editor/languages/definitions/ini/register";
import "monaco-editor/languages/definitions/javascript/register";
import "monaco-editor/languages/definitions/markdown/register";
import "monaco-editor/languages/definitions/python/register";
import "monaco-editor/languages/definitions/rust/register";
import "monaco-editor/languages/definitions/shell/register";
import "monaco-editor/languages/definitions/typescript/register";
import "monaco-editor/languages/definitions/yaml/register";
import EditorWorker from "monaco-editor/editor/editor.worker?worker";
import JsonWorker from "monaco-editor/languages/features/json/json.worker?worker";

self.MonacoEnvironment = {
  getWorker(_id: string, label: string) {
    return label === "json" ? new JsonWorker() : new EditorWorker();
  },
};

loader.config({ monaco });
