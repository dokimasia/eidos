// Shaped has a property, which a method of a double cannot implement.
export interface Shaped {
  name: string;
  get(): string;
}
