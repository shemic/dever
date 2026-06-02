const ReactDOMClient = (window as any).ReactDOMClient || {};

export const createRoot = ReactDOMClient.createRoot;
export const hydrateRoot = ReactDOMClient.hydrateRoot;
export const version = ReactDOMClient.version;

export default ReactDOMClient;
