import { ListEntries, CreateEntry, DeleteEntry } from '../../../wailsjs/go/api/DNSService.js';
import { List as ListTLDs } from '../../../wailsjs/go/api/TLDService.js';

let entries = $state([]);
let tlds = $state([]);

export function getEntries() {
  return entries;
}

export function getTlds() {
  return tlds;
}

export async function loadAll() {
  try {
    const [e, t] = await Promise.all([ListEntries(''), ListTLDs()]);
    entries = e || [];
    tlds = t || [];
  } catch {
    entries = [];
    tlds = [];
  }
}

export async function addEntry(domain, tld, target) {
  await CreateEntry(domain, tld, target || '127.0.0.1');
  await loadAll();
}

export async function removeEntry(domain) {
  await DeleteEntry(domain);
  await loadAll();
}
