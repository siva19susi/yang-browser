import { error } from '@sveltejs/kit'

export async function load({ url, params }) {
  const kind = params.kind
  const basename = params.basename

  if(kind !== "local" && kind !== "nsp") {
    throw error(404, "Unsupported kind")
  }

  const urlPath = url.searchParams.get("path")?.trim() ?? "" 
  return { kind, basename, urlPath }
}