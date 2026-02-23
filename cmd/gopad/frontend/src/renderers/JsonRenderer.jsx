export default function JsonRenderer({ data }) {
  if (data === null || data === undefined) {
    return <div className="rich-empty">No JSON data</div>;
  }

  if (typeof data === "object" && !Array.isArray(data)) {
    return (
      <div className="rich-json-card">
        {Object.entries(data).map(([key, value]) => (
          <div key={key} className="json-card-row">
            <span className="json-card-key">{key}</span>
            <span className={`json-card-value ${valueClass(value)}`}>
              {formatValue(value)}
            </span>
          </div>
        ))}
      </div>
    );
  }

  return (
    <pre className="rich-json-raw">{JSON.stringify(data, null, 2)}</pre>
  );
}

function valueClass(value) {
  if (value === null) return "json-null";
  if (typeof value === "boolean") return "json-bool";
  if (typeof value === "number") return "json-number";
  if (typeof value === "string") return "json-string";
  return "json-object";
}

function formatValue(value) {
  if (value === null) return "null";
  if (typeof value === "boolean") return String(value);
  if (typeof value === "number") return String(value);
  if (typeof value === "string") return `"${value}"`;
  return JSON.stringify(value, null, 2);
}
