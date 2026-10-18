/**
 * Utilities for inspecting and resolving Docker Compose YAML path references.
 */

/**
 * Detects any relative volume host paths in a Compose YAML string (e.g. ./data, ../config, .).
 */
export function detectRelativeVolumes(yaml: string): string[] {
  if (!yaml) return [];
  const lines = yaml.split('\n');
  const relPaths = new Set<string>();

  for (const line of lines) {
    // Short syntax: - ./data:/var/lib/gitea or - "./data:/var/lib/gitea"
    const shortMatch = line.match(/^\s*-\s*["']?(\.{1,2}(?:\/[^:\s"']*)?|\.)(?::[^\n\r]+)?["']?\s*(?:#.*)?$/);
    if (shortMatch) {
      relPaths.add(shortMatch[1]);
      continue;
    }
    // Long syntax: source: ./data or source: "./data"
    const longMatch = line.match(/^\s*source:\s*["']?(\.{1,2}(?:\/[^\s"']*)?|\.)["']?\s*(?:#.*)?$/);
    if (longMatch) {
      relPaths.add(longMatch[1]);
    }
  }

  return Array.from(relPaths);
}

/**
 * Normalizes a base directory and relative path into an absolute path.
 */
export function normalizePath(baseDir: string, relPath: string): string {
  const cleanBase = baseDir.trim().replace(/\/+$/, '');
  const parts = cleanBase.split('/').filter(Boolean);
  const relParts = relPath.split('/').filter(Boolean);

  for (const p of relParts) {
    if (p === '.') continue;
    if (p === '..') {
      parts.pop();
    } else {
      parts.push(p);
    }
  }

  return '/' + parts.join('/');
}

/**
 * Resolves all relative volume paths in a Compose YAML string using a base directory.
 */
export function resolveRelativeVolumesInYaml(yaml: string, baseDir: string): string {
  if (!yaml || !baseDir) return yaml;
  const cleanBase = baseDir.trim().replace(/\/+$/, '');
  if (!cleanBase) return yaml;

  const lines = yaml.split('\n');
  return lines.map(line => {
    // Short syntax: - ./data:/var/lib/gitea or - "./data:/var/lib/gitea"
    const shortMatch = line.match(/^(\s*-\s*["']?)(\.{1,2}(?:\/[^:\s"']*)?|\.)((?::[^"\n\r]+)?["']?\s*(?:#.*)?)$/);
    if (shortMatch) {
      const prefix = shortMatch[1];
      const rel = shortMatch[2];
      const suffix = shortMatch[3];
      const resolved = normalizePath(cleanBase, rel);
      return `${prefix}${resolved}${suffix}`;
    }

    // Long syntax: source: ./data or source: "./data"
    const longMatch = line.match(/^(\s*source:\s*["']?)(\.{1,2}(?:\/[^\s"']*)?|\.)(["']?\s*(?:#.*)?)$/);
    if (longMatch) {
      const prefix = longMatch[1];
      const rel = longMatch[2];
      const suffix = longMatch[3];
      const resolved = normalizePath(cleanBase, rel);
      return `${prefix}${resolved}${suffix}`;
    }

    return line;
  }).join('\n');
}

/**
 * Extracts the host directory where a stack's Compose file is located.
 */
export function getStackComposeDir(stack: { working_dir?: string; config_file?: string } | null): string | null {
  if (!stack) return null;
  if (stack.working_dir && stack.working_dir.trim()) {
    return stack.working_dir.trim().replace(/\/+$/, '');
  }
  if (stack.config_file && stack.config_file.trim()) {
    const trimmed = stack.config_file.trim();
    const lastSlash = trimmed.lastIndexOf('/');
    if (lastSlash > 0) {
      return trimmed.slice(0, lastSlash);
    }
  }
  return null;
}
