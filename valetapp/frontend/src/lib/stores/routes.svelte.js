import { List, Create, Update, Delete, ListTemplates, Preview } from '../../../wailsjs/go/api/RouteService.js';

let routes = $state([]);
let templates = $state([]);

export function getRoutes() {
  return routes;
}

export function getTemplates() {
  return templates;
}

export async function loadRoutes() {
  try {
    routes = await List();
  } catch {
    routes = [];
  }
}

export async function loadTemplates() {
  try {
    templates = await ListTemplates();
  } catch {
    templates = [];
  }
}

export async function addRoute(req) {
  await Create(req);
  await loadRoutes();
}

export async function updateRoute(id, req) {
  await Update(id, req);
  await loadRoutes();
}

export async function deleteRoute(id) {
  await Delete(id);
  await loadRoutes();
}

export async function previewRoute(req) {
  return await Preview(req);
}
