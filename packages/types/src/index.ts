// The package's whole surface: the three types openapi-typescript generates
// from the OpenAPI document. Nothing here is hand-written, and nothing may be —
// `make check-contract` regenerates both files and fails on any difference.
export type { paths, components, operations } from './api'
