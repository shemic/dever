import fs from "node:fs";
import ts from "typescript";

export type FrontSDKExports = {
  coreExports: ReadonlySet<string>;
  compatExports: Readonly<Record<string, string>>;
};

export function readFrontSDKExports(
  coreEntry: string,
  sdkEntry: string,
): FrontSDKExports {
  return {
    coreExports: readRuntimeExports(coreEntry),
    compatExports: readCompatExports(sdkEntry),
  };
}

function readRuntimeExports(file: string) {
  const sourceFile = readSourceFile(file);
  const exports = new Set<string>();

  for (const statement of sourceFile.statements) {
    if (
      (ts.isFunctionDeclaration(statement) ||
        ts.isClassDeclaration(statement) ||
        ts.isEnumDeclaration(statement)) &&
      statement.name &&
      hasExportModifier(statement)
    ) {
      exports.add(statement.name.text);
      continue;
    }
    if (ts.isVariableStatement(statement) && hasExportModifier(statement)) {
      for (const declaration of statement.declarationList.declarations) {
        collectBindingNames(declaration.name, exports);
      }
      continue;
    }
    if (
      ts.isExportDeclaration(statement) &&
      !statement.isTypeOnly &&
      !statement.moduleSpecifier &&
      statement.exportClause &&
      ts.isNamedExports(statement.exportClause)
    ) {
      for (const element of statement.exportClause.elements) {
        if (!element.isTypeOnly) {
          exports.add(element.name.text);
        }
      }
    }
  }
  return exports;
}

function readCompatExports(file: string) {
  const sourceFile = readSourceFile(file);
  const compatModules = new Map<string, string>();
  const compatExports: Record<string, string> = {};

  for (const statement of sourceFile.statements) {
    if (!ts.isVariableStatement(statement)) {
      continue;
    }
    for (const declaration of statement.declarationList.declarations) {
      if (!ts.isIdentifier(declaration.name) || !declaration.initializer) {
        continue;
      }
      const source = compatModuleSource(declaration.initializer, compatModules);
      if (source) {
        compatModules.set(declaration.name.text, source);
      }
    }
  }

  for (const statement of sourceFile.statements) {
    if (!ts.isVariableStatement(statement) || !hasExportModifier(statement)) {
      continue;
    }
    for (const declaration of statement.declarationList.declarations) {
      if (!ts.isIdentifier(declaration.name) || !declaration.initializer) {
        continue;
      }
      const binding = compatExportBinding(
        declaration.initializer,
        compatModules,
      );
      if (!binding) {
        continue;
      }
      const exportName = declaration.name.text;
      if (binding.memberName !== exportName) {
        throw new Error(
          `[dever-front-plugin] ${file} 的兼容导出 ${exportName} 必须映射同名宿主成员，当前为 ${binding.memberName}`,
        );
      }
      compatExports[exportName] = binding.source;
    }
  }
  return compatExports;
}

function compatExportBinding(
  expression: ts.Expression,
  compatModules: ReadonlyMap<string, string>,
) {
  const unwrapped = unwrapExpression(expression);
  if (ts.isPropertyAccessExpression(unwrapped)) {
    const source = compatModuleSource(unwrapped.expression, compatModules);
    return source
      ? { source, memberName: unwrapped.name.text }
      : undefined;
  }
  if (
    ts.isElementAccessExpression(unwrapped) &&
    unwrapped.argumentExpression &&
    ts.isStringLiteralLike(unwrapped.argumentExpression)
  ) {
    const source = compatModuleSource(unwrapped.expression, compatModules);
    return source
      ? { source, memberName: unwrapped.argumentExpression.text }
      : undefined;
  }
  return undefined;
}

function compatModuleSource(
  expression: ts.Expression,
  compatModules: ReadonlyMap<string, string>,
) {
  const unwrapped = unwrapExpression(expression);
  if (ts.isIdentifier(unwrapped)) {
    return compatModules.get(unwrapped.text);
  }
  if (
    !ts.isCallExpression(unwrapped) ||
    !ts.isIdentifier(unwrapped.expression) ||
    unwrapped.expression.text !== "getCompatModule" ||
    unwrapped.arguments.length !== 1
  ) {
    return undefined;
  }
  const source = unwrapped.arguments[0];
  return ts.isStringLiteralLike(source) ? source.text.trim() : undefined;
}

function unwrapExpression(expression: ts.Expression): ts.Expression {
  if (
    ts.isParenthesizedExpression(expression) ||
    ts.isAsExpression(expression) ||
    ts.isTypeAssertionExpression(expression) ||
    ts.isSatisfiesExpression(expression) ||
    ts.isNonNullExpression(expression)
  ) {
    return unwrapExpression(expression.expression);
  }
  return expression;
}

function collectBindingNames(name: ts.BindingName, names: Set<string>) {
  if (ts.isIdentifier(name)) {
    names.add(name.text);
    return;
  }
  for (const element of name.elements) {
    if (!ts.isOmittedExpression(element)) {
      collectBindingNames(element.name, names);
    }
  }
}

function hasExportModifier(node: ts.Node) {
  return Boolean(
    ts.canHaveModifiers(node) &&
      ts
        .getModifiers(node)
        ?.some((modifier) => modifier.kind === ts.SyntaxKind.ExportKeyword),
  );
}

function readSourceFile(file: string) {
  return ts.createSourceFile(
    file,
    fs.readFileSync(file, "utf8"),
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TS,
  );
}
