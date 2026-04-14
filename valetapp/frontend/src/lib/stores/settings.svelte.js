import { GetAll, Set } from '../../../wailsjs/go/api/SettingsService.js';
import { applyTheme, applyFont } from '../themes.js';

let settings = $state({});
let loaded = $state(false);

export function getSettings() { return settings; }
export function isLoaded() { return loaded; }

export async function loadSettings() {
  try {
    settings = await GetAll();
  } catch (e) {
    console.error('Failed to load settings:', e);
    settings = {};
  }
  loaded = true;
  applyTheme(settings.theme || 'macos');
  applyFont(settings.font || 'system');
}

export async function setSetting(key, value) {
  await Set(key, value);
  settings = { ...settings, [key]: value };
  if (key === 'theme') applyTheme(value);
  if (key === 'font') applyFont(value);
}
