// Hintable is the set of types that we can de/serialize with hints.
//
// When we deserialize objects from a typeless format, such as a query string,
// we can use a hint object to figure out how to deserialize the value.
//
// For example "a=1" could be deserialized as {a:'1'} or {a:1}, but if we
// provide a hint object, e.g. {a:100}, the deserializer can look at the type of
// the value in the hint and use that to guide the deserialization to correctly
// choose {a:1}.
export type Hintable = number | boolean | string | any[] | HintableObject;

// HintableObject is any object with strings for keys and only contains Hintable
// values.
export type HintableObject = { [key: string]: Hintable };
