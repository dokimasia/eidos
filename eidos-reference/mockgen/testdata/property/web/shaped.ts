// Shaped has a property, which a mock cannot implement.
//
//+reference:mock
export interface Shaped {
  name: string;
  get(): string;
}
