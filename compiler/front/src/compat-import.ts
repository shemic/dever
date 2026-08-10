import ts from "typescript";

export type CompatLoadMode = "virtual" | "module" | "preloaded";

type SourceEdit = {
  start: number;
  end: number;
  replacement: string;
};

type TransformOptions = {
  loadMode: CompatLoadMode;
  virtualModulePrefix?: string;
  sdkModuleSource?: string;
  sdkCoreSource?: string;
  sdkCoreExports?: ReadonlySet<string>;
  sdkCompatExports?: Readonly<Record<string, string>>;
  onCompatSource?: (source: string) => void;
  onImplicitCompatSource?: (source: string) => void;
};

type InlineCompatModule = {
  source: string;
  moduleName: string;
  bindings: string[];
};

type TransformContext = {
  id: string;
  mode: CompatLoadMode;
  names: () => string;
  virtualModulePrefix: string;
  inlineModules: Map<string, InlineCompatModule>;
};

export function rewriteCompatImports(
  code: string,
  id: string,
  options: TransformOptions,
) {
  const sourceFile = ts.createSourceFile(
    cleanModuleID(id),
    code,
    ts.ScriptTarget.Latest,
    true,
    scriptKind(id),
  );
  const context: TransformContext = {
    id,
    mode: options.loadMode,
    names: createUniqueNameFactory(code),
    virtualModulePrefix: options.virtualModulePrefix || "",
    inlineModules: new Map(),
  };
  if (context.mode === "virtual" && !context.virtualModulePrefix) {
    throw new Error(
      "[dever-front-plugin] virtual 兼容导入缺少 virtualModulePrefix",
    );
  }

  const edits: SourceEdit[] = [];
  const virtualCallModules = new Map<string, string>();

  for (const statement of sourceFile.statements) {
    if (ts.isImportDeclaration(statement)) {
      const source = moduleSource(statement.moduleSpecifier);
      if (source === options.sdkModuleSource) {
        const sdkImport = rewriteSDKImportDeclaration(
          statement,
          source,
          options,
          context,
        );
        if (sdkImport) {
          edits.push(replaceStatement(statement, sourceFile, sdkImport.code));
          sdkImport.compatSources.forEach((compatSource) =>
            options.onCompatSource?.(compatSource),
          );
          continue;
        }
      }
      if (!isCompatSource(source)) {
        continue;
      }

      const replacement = rewriteImportDeclaration(statement, source, context);
      edits.push(replaceStatement(statement, sourceFile, replacement.code));
      if (replacement.usesCompat) {
        options.onCompatSource?.(source);
      }
      continue;
    }

    if (ts.isExportDeclaration(statement)) {
      const source = moduleSource(statement.moduleSpecifier);
      if (!isCompatSource(source)) {
        continue;
      }
      const replacement = rewriteExportDeclaration(
        statement,
        source,
        sourceFile,
        context,
      );
      edits.push(replaceStatement(statement, sourceFile, replacement.code));
      if (replacement.usesCompat) {
        options.onCompatSource?.(source);
      }
    }
  }

  visitRuntimeImports(sourceFile, (node, source) => {
    if (node.arguments.length !== 1) {
      throw new Error(
        `[dever-front-plugin] ${cleanModuleID(id)} 的兼容动态导入不支持 import attributes`,
      );
    }
    edits.push({
      start: node.getStart(sourceFile),
      end: node.end,
      replacement: dynamicCompatImport(source, context),
    });
    options.onCompatSource?.(source);
  });

  visitCompatModuleReferences(sourceFile, (node, source) => {
    options.onCompatSource?.(source);
    if (!isGetCompatModuleCall(node)) {
      options.onImplicitCompatSource?.(source);
      return;
    }

    let moduleName: string;
    if (context.mode === "virtual") {
      moduleName = virtualCallModules.get(source) || "";
      if (!moduleName) {
        moduleName = context.names();
        virtualCallModules.set(source, moduleName);
      }
    } else {
      moduleName = inlineCompatModule(context, source).moduleName;
    }
    edits.push({
      start: node.getStart(sourceFile),
      end: node.end,
      replacement: moduleName,
    });
  });

  if (
    edits.length === 0 &&
    virtualCallModules.size === 0 &&
    context.inlineModules.size === 0
  ) {
    return null;
  }

  if (context.mode === "virtual" && virtualCallModules.size > 0) {
    const imports = Array.from(virtualCallModules, ([source, moduleName]) =>
      compatModuleImport(
        moduleName,
        compatVirtualModuleID(source, context.virtualModulePrefix),
      ),
    );
    edits.push({
      start: compatHeaderInsertionPoint(sourceFile, code),
      end: compatHeaderInsertionPoint(sourceFile, code),
      replacement: `\n${imports.join("\n")}\n`,
    });
  }

  if (context.mode !== "virtual" && context.inlineModules.size > 0) {
    const insertionPoint = compatHeaderInsertionPoint(sourceFile, code);
    edits.push({
      start: insertionPoint,
      end: insertionPoint,
      replacement: `\n${inlineCompatHeader(context)}\n`,
    });
  }

  return applySourceEdits(code, edits);
}

function rewriteSDKImportDeclaration(
  declaration: ts.ImportDeclaration,
  source: string,
  options: TransformOptions,
  context: TransformContext,
) {
  const clause = declaration.importClause;
  const bindings = clause?.namedBindings;
  if (clause && !clause.isTypeOnly && clause.name) {
    throw new Error(
      `[dever-front-plugin] ${cleanModuleID(context.id)} 不支持默认导入 ${source}；请改用命名导入。`,
    );
  }
  if (
    clause &&
    !clause.isTypeOnly &&
    bindings &&
    ts.isNamespaceImport(bindings)
  ) {
    throw new Error(
      `[dever-front-plugin] ${cleanModuleID(context.id)} 不支持命名空间导入 ${source}；请按实际需要使用命名导入。`,
    );
  }
  if (
    !clause ||
    !bindings ||
    !ts.isNamedImports(bindings) ||
    !options.sdkCoreSource ||
    !options.sdkCoreExports ||
    !options.sdkCompatExports
  ) {
    return null;
  }

  const coreElements: ts.ImportSpecifier[] = [];
  const fallbackElements: ts.ImportSpecifier[] = [];
  const compatElements = new Map<
    string,
    Array<{ importedName: string; localName: string }>
  >();

  for (const element of bindings.elements) {
    const importedName = (element.propertyName || element.name).text;
    const compatSource = options.sdkCompatExports[importedName];
    if (!clause.isTypeOnly && !element.isTypeOnly && compatSource) {
      const elements = compatElements.get(compatSource) || [];
      elements.push({ importedName, localName: element.name.text });
      compatElements.set(compatSource, elements);
    } else if (options.sdkCoreExports.has(importedName)) {
      coreElements.push(element);
    } else {
      fallbackElements.push(element);
    }
  }

  const unsupportedRuntimeExports = fallbackElements
    .filter((element) => !clause.isTypeOnly && !element.isTypeOnly)
    .map((element) => (element.propertyName || element.name).text);
  if (unsupportedRuntimeExports.length > 0) {
    throw new Error(
      `[dever-front-plugin] ${cleanModuleID(context.id)} 使用了未映射的 SDK 导出 ${unsupportedRuntimeExports.join(", ")}；请在 SDK index.ts 中声明对应宿主模块。`,
    );
  }
  if (compatElements.size === 0 && coreElements.length === 0) {
    return null;
  }

  const lines: string[] = [];
  if (coreElements.length > 0) {
    lines.push(
      namedSDKImport(options.sdkCoreSource, coreElements, clause.isTypeOnly),
    );
  }
  if (clause.name || fallbackElements.length > 0) {
    lines.push(
      sdkFallbackImport(
        source,
        clause.name?.text,
        fallbackElements,
        clause.isTypeOnly,
      ),
    );
  }
  for (const [compatSource, elements] of compatElements) {
    lines.push(
      ...withCompatModule(context, compatSource, (moduleName) =>
        elements.map(
          (element) =>
            `const ${element.localName} = ${moduleName}[${JSON.stringify(element.importedName)}];`,
        ),
      ),
    );
  }

  return {
    code: lines.join("\n"),
    compatSources: Array.from(compatElements.keys()),
  };
}

function rewriteImportDeclaration(
  declaration: ts.ImportDeclaration,
  source: string,
  context: TransformContext,
) {
  const clause = declaration.importClause;
  if (clause?.isTypeOnly) {
    return { code: "", usesCompat: false };
  }
  if (!clause) {
    if (context.mode === "virtual") {
      return {
        code: `import ${JSON.stringify(compatVirtualModuleID(source, context.virtualModulePrefix))};`,
        usesCompat: true,
      };
    }
    inlineCompatModule(context, source);
    return { code: "", usesCompat: true };
  }

  const bindingLines = (moduleName: string) => {
    const lines: string[] = [];
    if (clause.name) {
      lines.push(`const ${clause.name.text} = ${moduleName}.default;`);
    }
    const bindings = clause.namedBindings;
    if (bindings && ts.isNamespaceImport(bindings)) {
      lines.push(`const ${bindings.name.text} = ${moduleName};`);
    } else if (bindings) {
      for (const element of bindings.elements) {
        if (element.isTypeOnly) {
          continue;
        }
        const importedName = (element.propertyName || element.name).text;
        lines.push(
          `const ${element.name.text} = ${moduleName}[${JSON.stringify(importedName)}];`,
        );
      }
    }
    return lines;
  };
  const hasRuntimeBinding = bindingLines("module").length > 0;
  if (!hasRuntimeBinding) {
    return { code: "", usesCompat: false };
  }
  return {
    code: withCompatModule(context, source, bindingLines).join("\n"),
    usesCompat: true,
  };
}

function rewriteExportDeclaration(
  declaration: ts.ExportDeclaration,
  source: string,
  sourceFile: ts.SourceFile,
  context: TransformContext,
) {
  if (declaration.isTypeOnly) {
    return { code: "", usesCompat: false };
  }
  if (!declaration.exportClause) {
    throw new Error(
      `[dever-front-plugin] ${cleanModuleID(context.id)} 不支持从 ${JSON.stringify(source)} 使用 export *；请改为显式命名导出。`,
    );
  }
  const exportClause = declaration.exportClause;
  if (
    ts.isNamedExports(exportClause) &&
    exportClause.elements.every((element) => element.isTypeOnly)
  ) {
    return { code: "", usesCompat: false };
  }

  const bindingLines = (moduleName: string) => {
    const lines: string[] = [];
    const exports: string[] = [];
    if (ts.isNamespaceExport(exportClause)) {
      const exportName = exportClause.name.getText(sourceFile);
      const localName = context.names();
      lines.push(`const ${localName} = ${moduleName};`);
      exports.push(`${localName} as ${exportName}`);
    } else {
      for (const element of exportClause.elements) {
        if (element.isTypeOnly) {
          continue;
        }
        const importedName = (element.propertyName || element.name).text;
        const exportName = element.name.getText(sourceFile);
        const localName = context.names();
        lines.push(
          `const ${localName} = ${moduleName}[${JSON.stringify(importedName)}];`,
        );
        exports.push(`${localName} as ${exportName}`);
      }
    }
    if (exports.length > 0) {
      lines.push(`export { ${exports.join(", ")} };`);
    }
    return lines;
  };
  const lines = withCompatModule(context, source, bindingLines);
  return {
    code: lines.join("\n"),
    usesCompat: lines.length > 0 || context.mode !== "virtual",
  };
}

function withCompatModule(
  context: TransformContext,
  source: string,
  createBindings: (moduleName: string) => string[],
) {
  if (context.mode === "virtual") {
    const moduleName = context.names();
    const bindings = createBindings(moduleName);
    if (bindings.length === 0) {
      return [];
    }
    return [
      compatModuleImport(
        moduleName,
        compatVirtualModuleID(source, context.virtualModulePrefix),
      ),
      ...bindings,
    ];
  }

  const compatModule = inlineCompatModule(context, source);
  const bindings = createBindings(compatModule.moduleName);
  compatModule.bindings.push(...bindings);
  return [];
}

function inlineCompatModule(context: TransformContext, source: string) {
  let compatModule = context.inlineModules.get(source);
  if (!compatModule) {
    compatModule = {
      source,
      moduleName: context.names(),
      bindings: [],
    };
    context.inlineModules.set(source, compatModule);
  }
  return compatModule;
}

function inlineCompatHeader(context: TransformContext) {
  const modules = Array.from(context.inlineModules.values());
  const lines: string[] = [];
  if (context.mode === "module") {
    lines.push(
      `await window.DeverFront?.ensureCompat?.(${JSON.stringify(modules.map((item) => item.source))});`,
    );
  }
  for (const compatModule of modules) {
    lines.push(
      ...readCompatModuleLines(compatModule.source, compatModule.moduleName),
      ...compatModule.bindings,
    );
  }
  return lines.join("\n");
}

function dynamicCompatImport(source: string, context: TransformContext) {
  if (context.mode === "virtual") {
    const virtualModule = compatVirtualModuleID(
      source,
      context.virtualModulePrefix,
    );
    return `import(${JSON.stringify(virtualModule)}).then((module) => module.default)`;
  }

  const moduleName = context.names();
  const ensure =
    context.mode === "module"
      ? `window.DeverFront?.ensureCompat?.([${JSON.stringify(source)}])`
      : "";
  return [
    `Promise.resolve(${ensure}).then(() => {`,
    ...readCompatModuleLines(source, moduleName, "  "),
    `  return ${moduleName};`,
    "})",
  ].join("\n");
}

function readCompatModuleLines(
  source: string,
  moduleName: string,
  indent = "",
) {
  const message = `[dever-front-plugin] 宿主未注册兼容模块 ${source}`;
  return [
    `${indent}const ${moduleName} = window.DeverFront?.sdk?.getCompatModule(${JSON.stringify(source)});`,
    `${indent}if (!${moduleName} || Object.keys(${moduleName}).length === 0) {`,
    `${indent}  throw new Error(${JSON.stringify(message)});`,
    `${indent}}`,
  ];
}

function namedSDKImport(
  source: string,
  elements: ts.ImportSpecifier[],
  typeOnly: boolean,
) {
  return `import${typeOnly ? " type" : ""} { ${elements.map(formatImportSpecifier).join(", ")} } from ${JSON.stringify(source)};`;
}

function sdkFallbackImport(
  source: string,
  defaultName: string | undefined,
  elements: ts.ImportSpecifier[],
  typeOnly: boolean,
) {
  const named =
    elements.length > 0
      ? `{ ${elements.map(formatImportSpecifier).join(", ")} }`
      : "";
  const bindings = [defaultName || "", named].filter(Boolean).join(", ");
  return `import${typeOnly ? " type" : ""} ${bindings} from ${JSON.stringify(source)};`;
}

function formatImportSpecifier(element: ts.ImportSpecifier) {
  const importedName = (element.propertyName || element.name).text;
  const localName = element.name.text;
  const binding =
    importedName === localName
      ? importedName
      : `${importedName} as ${localName}`;
  return `${element.isTypeOnly ? "type " : ""}${binding}`;
}

function visitRuntimeImports(
  sourceFile: ts.SourceFile,
  visit: (node: ts.CallExpression, source: string) => void,
) {
  const walk = (node: ts.Node) => {
    if (
      ts.isCallExpression(node) &&
      node.expression.kind === ts.SyntaxKind.ImportKeyword
    ) {
      const source = moduleSource(node.arguments[0]);
      if (isCompatSource(source)) {
        visit(node, source);
        return;
      }
    }
    ts.forEachChild(node, walk);
  };
  walk(sourceFile);
}

function visitCompatModuleReferences(
  sourceFile: ts.SourceFile,
  visit: (node: ts.CallExpression, source: string) => void,
) {
  const walk = (node: ts.Node) => {
    if (
      ts.isCallExpression(node) &&
      node.expression.kind !== ts.SyntaxKind.ImportKeyword
    ) {
      const source = moduleSource(node.arguments[0]);
      if (isCompatSource(source)) {
        visit(node, source);
      }
    }
    ts.forEachChild(node, walk);
  };
  walk(sourceFile);
}

function isGetCompatModuleCall(node: ts.CallExpression) {
  const expression = node.expression;
  if (ts.isIdentifier(expression)) {
    return expression.text === "getCompatModule";
  }
  return (
    ts.isPropertyAccessExpression(expression) &&
    expression.name.text === "getCompatModule"
  );
}

function compatHeaderInsertionPoint(sourceFile: ts.SourceFile, code: string) {
  let insertionPoint = shebangEnd(code);
  for (const statement of sourceFile.statements) {
    if (ts.isImportDeclaration(statement)) {
      insertionPoint = Math.max(insertionPoint, statement.end);
    }
  }
  if (insertionPoint > shebangEnd(code)) {
    return insertionPoint;
  }

  for (const statement of sourceFile.statements) {
    if (!isDirectiveStatement(statement)) {
      break;
    }
    insertionPoint = statement.end;
  }
  if (insertionPoint === 0 && sourceFile.statements.length > 0) {
    return sourceFile.statements[0].getStart(sourceFile);
  }
  return insertionPoint;
}

function isDirectiveStatement(statement: ts.Statement) {
  return (
    ts.isExpressionStatement(statement) &&
    (ts.isStringLiteral(statement.expression) ||
      ts.isNoSubstitutionTemplateLiteral(statement.expression))
  );
}

function shebangEnd(code: string) {
  if (!code.startsWith("#!")) {
    return 0;
  }
  const lineEnd = code.indexOf("\n");
  return lineEnd === -1 ? code.length : lineEnd + 1;
}

function replaceStatement(
  statement: ts.Statement,
  sourceFile: ts.SourceFile,
  replacement: string,
): SourceEdit {
  return {
    start: statement.getStart(sourceFile),
    end: statement.end,
    replacement,
  };
}

function compatVirtualModuleID(source: string, virtualModulePrefix: string) {
  return virtualModulePrefix + source;
}

function compatModuleImport(moduleName: string, virtualSource: string) {
  return `import ${moduleName} from ${JSON.stringify(virtualSource)};`;
}

function moduleSource(node: ts.Expression | undefined) {
  if (!node) {
    return "";
  }
  if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) {
    return node.text;
  }
  return "";
}

function isCompatSource(source: string) {
  return source.startsWith("@/");
}

function createUniqueNameFactory(code: string) {
  let sequence = 0;
  const allocated = new Set<string>();
  return () => {
    let name = "";
    do {
      name = `__deverCompatModule${sequence++}`;
    } while (allocated.has(name) || code.includes(name));
    allocated.add(name);
    return name;
  };
}

function applySourceEdits(code: string, edits: SourceEdit[]) {
  const ordered = [...edits].sort((left, right) => left.start - right.start);
  for (let index = 1; index < ordered.length; index += 1) {
    if (ordered[index].start < ordered[index - 1].end) {
      throw new Error("[dever-front-plugin] 兼容导入转换产生了重叠修改");
    }
  }

  let result = code;
  for (const edit of ordered.reverse()) {
    result =
      result.slice(0, edit.start) + edit.replacement + result.slice(edit.end);
  }
  return result;
}

function cleanModuleID(id: string) {
  return id.split("?", 1)[0];
}

function scriptKind(id: string) {
  const cleanID = cleanModuleID(id).toLowerCase();
  if (cleanID.endsWith(".tsx")) {
    return ts.ScriptKind.TSX;
  }
  if (cleanID.endsWith(".jsx")) {
    return ts.ScriptKind.JSX;
  }
  if (
    cleanID.endsWith(".js") ||
    cleanID.endsWith(".mjs") ||
    cleanID.endsWith(".cjs")
  ) {
    return ts.ScriptKind.JS;
  }
  return ts.ScriptKind.TS;
}
