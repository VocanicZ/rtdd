export function add(a: number, b: number): number {
  return a + b;
}

export const sum = async (ns: number[]) => {
  let total = 0;
  for (const n of ns) {
    total = add(total, n);
  }
  return total;
};

const twice = function (n: number) {
  return add(n, n);
};

export class Acc {
  private total = 0;

  push(n: number): void {
    this.total = add(this.total, n);
  }

  static of(ns: number[]): Acc {
    const acc = new Acc();
    ns.forEach((n) => acc.push(n));
    return acc;
  }
}
