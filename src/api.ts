import createClient from 'openapi-fetch';
import type { components, paths } from './api-schema';

export type EditRequest = components['schemas']['EditRequest'];

export const apiClient = createClient<paths>({
  baseUrl: import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8000',
});

