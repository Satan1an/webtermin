// Serve Monaco from our own bundle. @monaco-editor/react otherwise fetches it
// from cdn.jsdelivr.net, which the panel's CSP (script-src 'self') blocks and
// which an offline LAN box can't reach anyway. Imported only by the lazy
// Files/Stacks pages, so the editor stays out of the main chunk.
//
// Editor core + tokenizers for the languages FilesPage's guessLanguage maps,
// and the JSON language service. The full "monaco-editor" entry would also
// pull the TS/CSS/HTML services and their workers (~9 MB in the binary).
import { loader } from "@monaco-editor/react";
import "monaco-editor/esm/vs/editor/edcore.main";
import * as monaco from "monaco-editor/esm/vs/editor/editor.api.js";
import "monaco-editor/esm/vs/language/json/monaco.contribution";
import "monaco-editor/esm/vs/basic-languages/dockerfile/dockerfile.contribution";
import "monaco-editor/esm/vs/basic-languages/go/go.contribution";
import "monaco-editor/esm/vs/basic-languages/ini/ini.contribution";
import "monaco-editor/esm/vs/basic-languages/javascript/javascript.contribution";
import "monaco-editor/esm/vs/basic-languages/markdown/markdown.contribution";
import "monaco-editor/esm/vs/basic-languages/python/python.contribution";
import "monaco-editor/esm/vs/basic-languages/rust/rust.contribution";
import "monaco-editor/esm/vs/basic-languages/shell/shell.contribution";
import "monaco-editor/esm/vs/basic-languages/typescript/typescript.contribution";
import "monaco-editor/esm/vs/basic-languages/yaml/yaml.contribution";
import EditorWorker from "monaco-editor/esm/vs/editor/editor.worker?worker";
import JsonWorker from "monaco-editor/esm/vs/language/json/json.worker?worker";

self.MonacoEnvironment = {
  getWorker(_id: string, label: string) {
    return label === "json" ? new JsonWorker() : new EditorWorker();
  },
};

loader.config({ monaco });
