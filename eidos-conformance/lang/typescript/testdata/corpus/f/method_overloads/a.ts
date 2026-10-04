type int = number;

export class Box {
  Fill(x: int): void;
  Fill(): void;
  Fill(x?: int): void {}
}
