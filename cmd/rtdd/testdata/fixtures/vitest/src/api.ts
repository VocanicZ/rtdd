import { get } from "./store";

export function handle(k: string): string | undefined {
  return get(k);
}
