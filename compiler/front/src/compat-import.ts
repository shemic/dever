import ts from "typescript";

type SourceEdit = {
  start: number;
  end: number;
  replacement: string;
};

type TransformOptions = {
  virtualModulePrefix: string;
  rewriteCompatCalls?: boolean;
  sdkModuleSource?: string;
  sdkCoreSource?: string;
  sdkCoreExports?: ReadonlySet<string>;
  sdkCompatExports?: Readonly<Record<string, string>>;
  onCompatSource?: (source: string) => void;
  onImplicitCompatSource?: (source: string) => void;
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
  const names = createUniqueNameFactory(code);
  const edits: SourceEdit[] = [];
  const compatCallModules = new Map<string, string>();

  for (const statement of sourceFile.statements) {
    if (ts.isImportDeclaration(statement)) {
      const source = moduleSource(statement.moduleSpecifier);
      if (source === options.sdkModuleSource) {
        const sdkImport = rewriteSDKImportDeclaration(
          statement,
          source,
          id,
          options,
          names,
        );
        if (sdkImport) {
          edits.push({
            start: statement.getStart(sourceFile),
            end: statement.end,
            replacement: sdkImport.replacement,
          });
          sdkImport.compatSources.forEach((compatSource) => {
            options.onCompatSource?.(compatSource);
          });
          continue;
        }
      }
      if (!isCompatSource(source)) {
        continue;
      }
      const replacement = rewriteImportDeclaration(
        statement,
        source,
        options.virtualModulePrefix,
        names,
      );
      edits.push({
        start: statement.getStart(sourceFile),
        end: statement.end,
        replacement,
      });
      if (replacement) {
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
        options.virtualModulePrefix,
        names,
        id,
      );
      edits.push({
        start: statement.getStart(sourceFile),
        end: statement.end,
        replacement,
      });
      if (replacement) {
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
      replacement: `${dynamicCompatImport(source, options.virtualModulePrefix)}.then((module) => module.default)`,
    });
    options.onCompatSource?.(source);
  });

  visitCompatModuleReferences(sourceFile, (node, source) => {
    options.onCompatSource?.(source);
    if (!options.rewriteCompatCalls) {
      return;
    }
    // A real virtual import lets Rollup keep the dependency with the chunk
    // that uses it and tree-shake unused SDK convenience exports.
    if (!isGetCompatModuleCall(node)) {
      options.onImplicitCompatSource?.(source);
      return;
    }

    let moduleName = compatCallModules.get(source);
    if (!moduleName) {
      moduleName = names();
      compatCallModules.set(source, moduleName);
    }
    edits.push({
      start: node.getStart(sourceFile),
      end: node.end,
      replacement: moduleName,
    });
  });

  if (edits.length === 0 && compatCallModules.size === 0) {
    return null;
  }
  const rewritten = applySourceEdits(code, edits);
  if (compatCallModules.size === 0) {
    return rewritten;
  }

  const imports = Array.from(compatCallModules, ([source, moduleName]) =>
    compatModuleImport(
      moduleName,
      compatVirtualModuleID(source, options.virtualModulePrefix),
    ),
  );
  return `${imports.join("\n")}\n${rewritten}`;
}

function rewriteSDKImportDeclaration(
  declaration: ts.ImportDeclaration,
  source: string,
  id: string,
  options: TransformOptions,
  names: () => string,
) {
  const clause = declaration.importClause;
  const bindings = clause?.namedBindings;
  if (clause && !clause.isTypeOnly && clause.name) {
    throw new Error(
      `[dever-front-plugin] ${cleanModuleID(id)} 不支持默认导入 ${source}；请改用命名导入。`,
    );
  }
  if (
    clause &&
    !clause.isTypeOnly &&
    bindings &&
    ts.isNamespaceImport(bindings)
  ) {
    throw new Error(
      `[dever-front-plugin] ${cleanModuleID(id)} 不支持命名空间导入 ${source}；请按实际需要使用命名导入。`,
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
      `[dever-front-plugin] ${cleanModuleID(id)} 使用了未映射的 SDK 导出 ${unsupportedRuntimeExports.join(", ")}；请在 SDK index.ts 中声明对应宿主模块。`,
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
    const moduleName = names();
    lines.push(
      compatModuleImport(
        moduleName,
        compatVirtualModuleID(compatSource, options.virtualModulePrefix),
      ),
    );
    for (const element of elements) {
      lines.push(
        `const ${element.localName} = ${moduleName}[${JSON.stringify(element.importedName)}];`,
      );
    }
  }

  return {
    replacement: lines.join("\n"),
    compatSources: Array.from(compatElements.keys()),
  };
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

function rewriteImportDeclaration(
  declaration: ts.ImportDeclaration,
  source: string,
  virtualModulePrefix: string,
  names: () => string,
) {
  const clause = declaration.importClause;
  const virtualSource = compatVirtualModuleID(source, virtualModulePrefix);
  if (!clause) {
    return `import ${JSON.stringify(virtualSource)};`;
  }
  if (clause.isTypeOnly) {
    return "";
  }

  const moduleName = names();
  const lines = [compatModuleImport(moduleName, virtualSource)];
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

  if (lines.length === 1) {
    return "";
  }
  return lines.join("\n");
}

function rewriteExportDeclaration(
  declaration: ts.ExportDeclaration,
  source: string,
  sourceFile: ts.SourceFile,
  virtualModulePrefix: string,
  names: () => string,
  id: string,
) {
  if (declaration.isTypeOnly) {
    return "";
  }
  if (!declaration.exportClause) {
    throw new Error(
      `[dever-front-plugin] ${cleanModuleID(id)} 不支持从 ${JSON.stringify(source)} 使用 export *；请改为显式命名导出。`,
    );
  }

  const moduleName = names();
  const virtualSource = compatVirtualModuleID(source, virtualModulePrefix);
  const lines = [compatModuleImport(moduleName, virtualSource)];
  const exports: string[] = [];

  if (ts.isNamespaceExport(declaration.exportClause)) {
    const exportName = declaration.exportClause.name.getText(sourceFile);
    const localName = names();
    lines.push(`const ${localName} = ${moduleName};`);
    exports.push(`${localName} as ${exportName}`);
  } else {
    for (const element of declaration.exportClause.elements) {
      if (element.isTypeOnly) {
        continue;
      }
      const importedName = (element.propertyName || element.name).text;
      const exportName = element.name.getText(sourceFile);
      const localName = names();
      lines.push(
        `const ${localName} = ${moduleName}[${JSON.stringify(importedName)}];`,
      );
      exports.push(`${localName} as ${exportName}`);
    }
  }

  if (exports.length === 0) {
    return "";
  }
  lines.push(`export { ${exports.join(", ")} };`);
  return lines.join("\n");
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

function dynamicCompatImport(source: string, virtualModulePrefix: string) {
  return `import(${JSON.stringify(compatVirtualModuleID(source, virtualModulePrefix))})`;
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
  if (cleanID.endsWith(".js") || cleanID.endsWith(".mjs")) {
    return ts.ScriptKind.JS;
  }
  if (cleanID.endsWith(".cjs")) {
    return ts.ScriptKind.JS;
  }
  return ts.ScriptKind.TS;
}
