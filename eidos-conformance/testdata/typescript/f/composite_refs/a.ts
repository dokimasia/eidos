import { Target } from "./dep/d";

export class Holder {
  f0: Target | undefined;
  f1!: Map<string, Target>;
  f2!: (t: Target) => Error;
}
