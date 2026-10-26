// class Phantom { load(id: string) { } }
export class Api {
  async load(
    id: string,
    retries: number,
  ): Promise<string> {
    return fetchOne(id, retries);
  }
}

export const fetchOne = (
  id: string,
  retries: number,
) => {
  return `id ${id}
  function ghost(x) {
  }`;
};
