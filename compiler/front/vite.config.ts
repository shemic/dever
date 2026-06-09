import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import react from "@vitejs/plugin-react-swc";
import { defineConfig, normalizePath, type PluginOption } from "vite";

const compilerRoot = path.dirname(fileURLToPath(import.meta.url));
const pluginRoot = process.env.DEVER_FRONT_PLUGIN_ROOT || "";
const pluginName = process.env.DEVER_FRONT_PLUGIN_NAME || "plugin";
const projectRoot =
  process.env.DEVER_FRONT_PLUGIN_PROJECT_ROOT ||
  path.resolve(compilerRoot, "..", "..", "..", "..");
const frontPackageRoot = resolveFrontPackageRoot();
const sdkEntry = path.resolve(frontPackageRoot, "sdk", "src", "index.ts");
const shimRoot = path.resolve(compilerRoot, "src", "shims");

const runtimeEntryID = "virtual:dever-front-plugin-runtime";
const resolvedRuntimeEntryID = "\0" + runtimeEntryID;
const pluginEntry = pluginRoot ? path.join(pluginRoot, "src", "plugin.ts") : "";

const compatModulePrefix = "virtual:dever-front-compat:";
const resolvedCompatModulePrefix = "\0" + compatModulePrefix;

const shimModuleFiles: Record<string, string> = {
  react: "react.ts",
  "react-jsx-runtime": "react-jsx-runtime.ts",
  "react-dom": "react-dom.ts",
  "react-dom-client": "react-dom-client.ts",
};

function resolveFrontPackageRoot() {
  const configured = process.env.DEVER_FRONT_PACKAGE_ROOT || "";
  if (hasFrontSDK(configured)) {
    return path.resolve(configured);
  }

  for (const candidate of [
    path.resolve(projectRoot, "package", "front"),
    path.resolve(projectRoot, "backend", "package", "front"),
  ]) {
    if (hasFrontSDK(candidate)) {
      return candidate;
    }
  }

  return path.resolve(projectRoot, "package", "front");
}

function hasFrontSDK(root: string) {
  if (!root) {
    return false;
  }
  return fs.existsSync(path.resolve(root, "sdk", "src", "index.ts"));
}

const pluginOptimizedDeps = [
  "@xyflow/react",
  "lucide-react",
  "sonner",
  "zustand",
  "zustand/react",
  "zustand/vanilla",
];

const compatExports: Record<string, string[]> = {
  "@/components/agent/interaction-panel": ["AgentInteractionPanel"],
  "@/components/agent/stream-request-params": [
    "PowerParamField",
    "PowerParamPopover",
    "normalizePowerParams",
    "normalizePowerParamConfig",
    "buildDefaultParamValues",
    "buildRequestInput",
    "validateMainParams",
    "paramFilesRequestValue",
    "summarizeParamDisplayValue",
    "inputKeyForParam",
    "isMainParam",
    "isToolbarParam",
    "isHiddenParam",
  ],
  "@/components/assistant/form-actions": [
    "AssistantFieldGenerateButton",
    "AssistantFormFillButton",
    "AssistantContextFormFillButton",
    "AssistantContextFieldGenerateButton",
  ],
  "@/components/assistant/reference-picker": [
    "AssistantReferencePicker",
    "AssistantReferenceList",
  ],
  "@/components/assistant/task-popover": ["AssistantTaskPopover"],
  "@/components/confirm-dialog": ["ConfirmDialog"],
  "@/components/energon/content-view": [
    "EnergonContentView",
    "normalizeEnergonOutput",
  ],
  "@/components/energon/progress": [
    "EnergonProgressBlock",
    "normalizeEnergonPercent",
  ],
  "@/components/layout/site-logo": ["SiteLogo"],
  "@/components/searchable-option-picker": ["SearchableOptionPicker"],
  "@/components/stream-timing": [
    "useStreamClock",
    "isStreamTimingRunning",
    "createStreamTiming",
    "createRuntimeStreamTiming",
    "updateStreamTiming",
    "updateStreamTimingFromOutput",
    "streamTimingStatusFromRuntimeStatus",
    "streamTimingPercentFromOutput",
    "isStreamTimingStatusOutput",
    "finishStreamTiming",
    "markStreamTimingStopping",
    "cancelStreamTiming",
    "StreamTimingBadge",
    "formatStreamDuration",
  ],
  "@/components/ui/button": ["Button", "buttonVariants"],
  "@/components/ui/card": [
    "Card",
    "CardHeader",
    "CardFooter",
    "CardTitle",
    "CardAction",
    "CardDescription",
    "CardContent",
  ],
  "@/components/ui/dialog": [
    "Dialog",
    "DialogClose",
    "DialogContent",
    "DialogDescription",
    "DialogFooter",
    "DialogHeader",
    "DialogOverlay",
    "DialogPortal",
    "DialogTitle",
    "DialogTrigger",
  ],
  "@/components/ui/input": ["Input"],
  "@/components/ui/radio-group": ["RadioGroup", "RadioGroupItem"],
  "@/components/ui/select": [
    "Select",
    "SelectContent",
    "SelectGroup",
    "SelectItem",
    "SelectLabel",
    "SelectScrollDownButton",
    "SelectScrollUpButton",
    "SelectSeparator",
    "SelectTrigger",
    "SelectValue",
  ],
  "@/components/ui/sheet": [
    "Sheet",
    "SheetTrigger",
    "SheetClose",
    "SheetContent",
    "SheetHeader",
    "SheetFooter",
    "SheetTitle",
    "SheetDescription",
  ],
  "@/components/ui/textarea": ["Textarea"],
  "@/config/app-config": [
    "getAppConfig",
    "getSiteConfig",
    "getAppearanceConfig",
    "getRuntimeConfig",
    "getDefaultSidebarOpen",
    "getDefaultCollapsibleMode",
  ],
  "@/hooks/use-upload-rule-metas": ["useUploadRuleMetas"],
  "@/lib/agent-result-protocol": [
    "normalizeAgentResultOutputValue",
    "extractAgentResultPayload",
    "isAgentResultProtocolText",
    "agentResultPayloadTitle",
  ],
  "@/lib/agent/runner": ["runAgentStream", "stopAgentStream"],
  "@/lib/assistant/context": [
    "buildAssistantPageContext",
    "assistantContextSummary",
    "buildAssistantFieldContext",
    "normalizeAssistantFormPath",
  ],
  "@/lib/assistant/reference": [
    "buildAssistantReferenceSummary",
    "buildAssistantReferenceMessage",
    "assistantReferencePayload",
    "normalizeAssistantReferences",
    "readAssistantReferenceFile",
    "uploadItemToAssistantReferenceFile",
    "resolveAssistantReferenceKind",
    "assistantReferenceKindText",
    "formatAssistantReferenceSize",
  ],
  "@/lib/auth-redirect": ["resolvePostLoginTarget"],
  "@/lib/icon": ["resolveLucideIcon"],
  "@/lib/page-schema-reload": ["reloadStorePageSchema"],
  "@/lib/plugin/types": ["defineFrontPlugin", "lazyNode", "mergePluginNodes"],
  "@/lib/request": [
    "REQUEST_ERROR_EVENT",
    "FRONT_RUNTIME_REFRESH_EVENT",
    "joinFrontApi",
    "joinSiteApi",
    "resolveRequestUrl",
    "resolveAssetUrl",
    "buildRuntimeRequestHeaders",
    "requestRaw",
    "request",
    "requestBlob",
    "loadPageSchema",
    "loadMainInfo",
    "loadAssistantPermissionContext",
    "resetFrontRuntimeCache",
    "loadSidebarMenu",
  ],
  "@/lib/resource": [
    "UNCATEGORIZED_RESOURCE_CATEGORY",
    "normalizeResourceSourceName",
    "normalizeResourceCategoryId",
    "listResources",
    "listResourceCategories",
    "listResourceSources",
    "assignResourceCategory",
    "assignResourceCategories",
    "buildFilterCategoryItems",
    "buildFilterSourceItems",
    "normalizeUploadItems",
    "normalizeUploadUrlItems",
    "resolveUploadSelectionKeys",
    "isSameUploadSelectionItem",
    "serializeUploadUrlItems",
    "normalizeUploadItem",
    "isImageResource",
    "isVideoResource",
    "isAudioResource",
    "resolveResourcePreviewKind",
    "isPreviewableResource",
    "formatUploadSize",
    "resolveResourceKind",
    "normalizeResourceUploadRules",
    "mergeResourceUploadRules",
    "resolveUploadActionLabel",
  ],
  "@/lib/runtime-stream-output": [
    "normalizeRuntimeFrameOutput",
    "isEmptyRuntimeOutput",
    "resolveRuntimeFrameCancelable",
    "runtimeErrorMessage",
    "isPlainRecord",
  ],
  "@/lib/runtime-stream-runner": [
    "runRuntimeStream",
    "watchRuntimeStream",
    "stopRuntimeStream",
  ],
  "@/lib/store": [
    "createPageStore",
    "PageStoreContext",
    "usePageStore",
    "usePageStoreValue",
    "useStorePathValue",
    "getStoreValueByPath",
    "setStoreValueByPath",
  ],
  "@/lib/stream": [
    "readRuntimeStreamFrame",
    "readRuntimeStreamEvents",
    "assertRuntimeStreamFrameSuccess",
    "streamValueText",
  ],
  "@/lib/utils": ["cn", "sleep", "getPageNumbers", "formatDisplayValue"],
  "@/page/nodes/show/tooltip": ["HoverTip", "InlineTip", "ShowTooltip"],
  "@/stores/auth-store": [
    "useAuthStore",
    "getAccessTokenKey",
    "getAuthUserKey",
  ],
};

function runtimeEntryPlugin(): PluginOption {
  return {
    name: "dever-front-plugin-runtime-entry",
    resolveId(id) {
      return id === runtimeEntryID ? resolvedRuntimeEntryID : null;
    },
    load(id) {
      if (id !== resolvedRuntimeEntryID) {
        return null;
      }
      return [
        `import plugin from ${JSON.stringify(normalizePath(pluginEntry))}`,
        "window.DeverFront?.registerPlugin(plugin)",
      ].join("\n");
    },
  };
}

function compatModulePlugin(): PluginOption {
  return {
    name: "dever-front-plugin-compat-modules",
    resolveId(id) {
      if (id.startsWith("@/")) {
        return resolvedCompatModulePrefix + id;
      }
      return null;
    },
    load(id) {
      if (!id.startsWith(resolvedCompatModulePrefix)) {
        return null;
      }

      const source = id.slice(resolvedCompatModulePrefix.length);
      const names = compatExports[source] || [];
      return [
        "import { getCompatModule } from '@dever/front-plugin'",
        `const mod = getCompatModule(${JSON.stringify(source)})`,
        ...names.map((name) => `export const ${name} = mod.${name}`),
        "export default mod.default",
      ].join("\n");
    },
  };
}

function dependency(name: string) {
  return path.resolve(compilerRoot, "node_modules", name);
}

function dependencyEntry(name: string) {
  return path.resolve(compilerRoot, "node_modules", ...name.split("/"));
}

function shimFile(name: string) {
  const file = shimModuleFiles[name];
  if (!file) {
    throw new Error(`Unknown front plugin shim: ${name}`);
  }
  return path.join(shimRoot, file);
}

function runtimeAlias(command: string) {
  const serve = command === "serve";
  return [
    { find: "@dever/front-plugin", replacement: sdkEntry },
    {
      find: /^zustand$/,
      replacement: dependencyEntry("zustand/esm/index.mjs"),
    },
    {
      find: /^zustand\/vanilla$/,
      replacement: dependencyEntry("zustand/esm/vanilla.mjs"),
    },
    {
      find: /^zustand\/react$/,
      replacement: dependencyEntry("zustand/esm/react.mjs"),
    },
    {
      find: "react/jsx-dev-runtime",
      replacement: serve
        ? shimFile("react-jsx-runtime")
        : dependency("react/jsx-dev-runtime.js"),
    },
    {
      find: "react/jsx-runtime",
      replacement: serve
        ? shimFile("react-jsx-runtime")
        : dependency("react/jsx-runtime.js"),
    },
    {
      find: "react-dom/client",
      replacement: serve
        ? shimFile("react-dom-client")
        : dependency("react-dom/client"),
    },
    {
      find: "react-dom",
      replacement: serve ? shimFile("react-dom") : dependency("react-dom"),
    },
    {
      find: "react",
      replacement: serve ? shimFile("react") : dependency("react"),
    },
    { find: "@xyflow/react", replacement: dependency("@xyflow/react") },
    { find: "lucide-react", replacement: dependency("lucide-react") },
    { find: "sonner", replacement: dependency("sonner") },
  ];
}

export default defineConfig(({ command }) => {
  return {
    root: projectRoot,
    plugins: [runtimeEntryPlugin(), compatModulePlugin(), react()],
    resolve: {
      dedupe: ["react", "react-dom"],
      alias: runtimeAlias(command),
    },
    server: {
      host: "127.0.0.1",
      hmr: false,
      fs: {
        allow: [projectRoot, frontPackageRoot, compilerRoot],
      },
    },
    optimizeDeps: {
      include: pluginOptimizedDeps,
    },
    build: {
      outDir: pluginRoot ? path.join(pluginRoot, "dist") : "dist",
      emptyOutDir: true,
      manifest: "manifest.json",
      lib: {
        entry: runtimeEntryID,
        formats: ["iife"],
        name: `${pluginName.replace(/[^a-zA-Z0-9_$]/g, "_")}FrontPlugin`,
        fileName: () => `${pluginName}.js`,
      },
      rollupOptions: {
        external: ["react"],
        output: {
          globals: {
            react: "React",
          },
          inlineDynamicImports: true,
        },
      },
    },
  };
});
