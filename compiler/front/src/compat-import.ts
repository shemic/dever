import ts from "typescript";

type SourceEdit = {
  start: number;
  end: number;
  replacement: string;
};

type TransformOptions = {
  virtualModulePrefix: string;
  onCompatSource?: (source: string) => void;
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
  visitCompatModuleReferences(sourceFile, options.onCompatSource);
  const names = createUniqueNameFactory(code);
  const edits: SourceEdit[] = [];

  for (const statement of sourceFile.statements) {
    if (ts.isImportDeclaration(statement)) {
      const source = moduleSource(statement.moduleSpecifier);
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

  if (edits.length === 0) {
    return null;
  }
  return applySourceEdits(code, edits);
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
  visit: ((source: string) => void) | undefined,
) {
  if (!visit) {
    return;
  }

  const walk = (node: ts.Node) => {
    if (ts.isCallExpression(node)) {
      const source = moduleSource(node.arguments[0]);
      if (isCompatSource(source)) {
        visit(source);
      }
    }
    ts.forEachChild(node, walk);
  };
  walk(sourceFile);
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
      result.slice(0, edit.start) +
      edit.replacement +
      result.slice(edit.end);
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
