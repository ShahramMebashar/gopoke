import TableRenderer from "./TableRenderer";
import JsonRenderer from "./JsonRenderer";
import FallbackRenderer from "./FallbackRenderer";

const registry = {
  table: TableRenderer,
  json: JsonRenderer,
};

export function getRenderer(type) {
  return registry[type] || FallbackRenderer;
}

export function registerRenderer(type, component) {
  registry[type] = component;
}
