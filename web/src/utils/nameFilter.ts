export function parseNameFilter(value: string): string[] {
  const seen = new Set<string>();
  const names: string[] = [];
  for (const part of value.split(/[,，]/)) {
    const name = part.trim();
    if (!name || seen.has(name)) {
      continue;
    }
    seen.add(name);
    names.push(name);
  }
  return names;
}

export function formatNameFilter(values: string[] | null | undefined): string {
  return (values || []).map((value) => value.trim()).filter(Boolean).join(', ');
}
