import { List, Create, Delete } from '../../../wailsjs/go/api/TLDService.js';

let tlds = $state([]);

export function getTlds() {
  return tlds;
}

export async function loadTlds() {
  try {
    tlds = await List();
  } catch {
    tlds = [];
  }
}

export async function addTld(tld) {
  await Create(tld);
  await loadTlds();
}

export async function deleteTld(tld) {
  await Delete(tld);
  await loadTlds();
}
