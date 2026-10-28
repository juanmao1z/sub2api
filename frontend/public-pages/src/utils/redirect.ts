// Extracted from frontend/src/views/auth/OAuthCallbackView.vue at
// 81025b8e210e3ddb33a8e78a8aa02aba0c3abc80, lines 242-249.
export function sanitizeRedirectPath(path: unknown): string {
  if (typeof path !== 'string' || !path) return '/home'
  if (!path.startsWith('/')) return '/home'
  if (path.startsWith('//')) return '/home'
  if (path.includes('://')) return '/home'
  if (/[\u0000-\u001f\u007f]/.test(path)) return '/home'
  // URL parsers normalize backslashes to slashes; reject that protocol-relative escape.
  if (path.includes('\\')) return '/home'
  return path
}
