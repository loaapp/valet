import { z } from 'zod';

const domainField = z.string()
  .min(1, 'Domain is required')
  .refine(s => !s.includes('://'), 'Do not include http:// or https://')
  .refine(s => !s.includes('/'), 'Do not include paths')
  .refine(s => !s.includes(' '), 'No spaces allowed')
  .refine(s => s === s.toLowerCase(), 'Must be lowercase')
  .refine(s => /^[a-z0-9]([a-z0-9.-]*[a-z0-9])?\.[a-z]+$/.test(s), 'Must be a valid domain (e.g., myapp.test)');

const upstreamField = z.string()
  .min(1, 'Upstream is required')
  .transform(s => s.replace(/^https?:\/\//i, ''))  // auto-strip protocol (case-insensitive)
  .pipe(z.string()
    .refine(s => !s.includes(' '), 'No spaces allowed')
    .refine(s => /^[a-zA-Z0-9._-]+:\d+$/.test(s), 'Must be host:port format (e.g., localhost:3000)')
  );

const upstreamOptional = z.string()
  .transform(s => s.replace(/^https?:\/\//i, '').trim())
  .pipe(z.string().refine(s => {
    if (!s) return true; // empty is OK for template mode
    return /^[a-zA-Z0-9._-]+:\d+$/.test(s);
  }, 'Must be host:port format (e.g., localhost:3000)'));

export const routeSchema = z.object({
  domain: domainField,
  upstream: upstreamField,
  description: z.string().optional(),
});

export const templateRouteSchema = z.object({
  domain: domainField,
  upstream: upstreamOptional,
  description: z.string().optional(),
});

const fieldSchemas = {
  domain: domainField,
  upstream: upstreamField,
  upstreamOptional: upstreamOptional,
};

// Validate a single field on blur. Returns error string or null.
export function validateField(field, value, useTemplate = false) {
  if (!value && field !== 'upstream') return null; // don't validate empty on blur (submit catches required)
  let schema = fieldSchemas[field];
  if (field === 'upstream' && useTemplate) schema = fieldSchemas.upstreamOptional;
  if (!schema) return null;
  const result = schema.safeParse(value);
  if (result.success) return null;
  return result.error.issues[0]?.message ?? 'Invalid value';
}

export function validateRoute(data, useTemplate = false) {
  const schema = useTemplate ? templateRouteSchema : routeSchema;
  const result = schema.safeParse(data);
  if (result.success) {
    return { valid: true, data: result.data, errors: {} };
  }
  const errors = {};
  result.error.issues.forEach(issue => {
    const field = issue.path[0];
    if (field && !errors[field]) {
      errors[field] = issue.message;
    }
  });
  return { valid: false, data: null, errors };
}
