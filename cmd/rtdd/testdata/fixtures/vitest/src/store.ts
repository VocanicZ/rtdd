const data: Record<string, string> = { a: "v:a" };

export function get(k: string): string | undefined {
  return data[k];
}
