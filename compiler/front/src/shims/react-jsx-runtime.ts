const React = (window as any).React;

export const Fragment = React.Fragment;

function withKey(props: any, key: any) {
  if (key === undefined || key === null) {
    return props || {};
  }
  return Object.assign({}, props || {}, { key });
}

export function jsx(type: any, props: any, key: any) {
  return React.createElement(type, withKey(props, key));
}

export const jsxs = jsx;

export function jsxDEV(type: any, props: any, key: any) {
  return React.createElement(type, withKey(props, key));
}

export default { Fragment, jsx, jsxs, jsxDEV };
