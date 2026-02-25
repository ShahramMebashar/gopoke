export default function TableRenderer({ data }) {
  if (!Array.isArray(data) || data.length === 0) {
    return <div className="rich-empty">No table data</div>;
  }

  const columns = Object.keys(data[0]);
  if (columns.length === 0) {
    return <div className="rich-empty">Empty table row</div>;
  }

  return (
    <div className="rich-table-wrap">
      <table className="rich-table">
        <thead>
          <tr>
            {columns.map((col) => (
              <th key={col}>{col}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {data.map((row, i) => (
            <tr key={i}>
              {columns.map((col) => (
                <td key={col}>{formatCell(row[col])}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function formatCell(value) {
  if (value === null || value === undefined) return "";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}
