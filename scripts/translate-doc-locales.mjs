#!/usr/bin/env node
import fs from 'node:fs/promises';
import path from 'node:path';

const ROOT = process.cwd();
const DOCS_ROOT = path.join(ROOT, 'docs');
const LOCALES_ROOT = path.join(DOCS_ROOT, 'locales');

const LOCALES = {
  ar: 'العربية',
  de: 'Deutsch',
  es: 'Español',
  fr: 'Français',
  hi: 'हिन्दी',
  id: 'Bahasa Indonesia',
  ja: '日本語',
  ko: '한국어',
  pt: 'Português',
  ru: 'Русский',
  vi: 'Tiếng Việt',
  zh: '中文',
};

const MYMEMORY_ENDPOINT = 'https://api.mymemory.translated.net/get';
const MAX_CHUNK_CHARS = 1000;

function usage() {
  console.error([
    'Usage:',
    '  node scripts/translate-doc-locales.mjs <locale|all> [relative-doc-path ...]',
    '',
    'Examples:',
    '  node scripts/translate-doc-locales.mjs ko sdk/app-module-guide.md',
    '  node scripts/translate-doc-locales.mjs all',
  ].join('\n'));
}

function isMarkdownFile(filePath) {
  return filePath.endsWith('.md');
}

async function walkMarkdownFiles(dir) {
  const entries = await fs.readdir(dir, { withFileTypes: true });
  const files = [];
  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      if (fullPath.startsWith(path.join(LOCALES_ROOT, ''))) {
        continue;
      }
      files.push(...await walkMarkdownFiles(fullPath));
      continue;
    }
    if (entry.isFile() && isMarkdownFile(fullPath)) {
      files.push(fullPath);
    }
  }
  return files;
}

function localeLabel(locale) {
  return LOCALES[locale] ?? locale;
}

function placeholderPrefix(kind, index) {
  return `__VEXO_${kind}_${index}__`;
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function protectInlineCode(text) {
  const placeholders = [];
  const protectedText = text.replace(/`[^`\n]+`/g, (match) => {
    const token = placeholderPrefix('CODE', placeholders.length);
    placeholders.push({ token, value: match });
    return token;
  });
  return {
    text: protectedText,
    restore(value) {
      let output = value;
      for (const { token, value: original } of placeholders) {
        output = output.split(token).join(original);
      }
      return output;
    },
  };
}

function protectCommonURLs(text) {
  const placeholders = [];
  const protectedText = text.replace(/https?:\/\/[^\s<>()\]]+/g, (match) => {
    const token = placeholderPrefix('URL', placeholders.length);
    placeholders.push({ token, value: match });
    return token;
  });
  return {
    text: protectedText,
    restore(value) {
      let output = value;
      for (const { token, value: original } of placeholders) {
        output = output.split(token).join(original);
      }
      return output;
    },
  };
}

function splitMarkdown(text) {
  const lines = text.split('\n');
  const chunks = [];
  let buffer = [];
  let fence = null;

  const flushBuffer = () => {
    if (buffer.length > 0) {
      chunks.push({ type: 'text', value: buffer.join('\n') });
      buffer = [];
    }
  };

  for (const line of lines) {
    const fenceMatch = line.match(/^(\s*)(```|~~~)/);
    if (fenceMatch) {
      if (!fence) {
        flushBuffer();
        fence = fenceMatch[2];
        buffer.push(line);
        continue;
      }
      if (fenceMatch[2] === fence) {
        buffer.push(line);
        chunks.push({ type: 'code', value: buffer.join('\n') });
        buffer = [];
        fence = null;
        continue;
      }
    }
    buffer.push(line);
  }

  if (buffer.length > 0) {
    chunks.push({ type: fence ? 'code' : 'text', value: buffer.join('\n') });
  }

  return chunks;
}

function splitParagraphs(text, maxChars = MAX_CHUNK_CHARS) {
  const paragraphs = text.split(/\n{2,}/);
  const chunks = [];
  let current = '';

  const pushCurrent = () => {
    if (current.trim()) {
      chunks.push(current);
    }
    current = '';
  };

  for (const paragraph of paragraphs) {
    if (paragraph.length > maxChars) {
      pushCurrent();
      const lines = paragraph.split('\n');
      let lineBuffer = '';
      for (const line of lines) {
        if (!lineBuffer) {
          lineBuffer = line;
          continue;
        }
        if ((lineBuffer.length + 1 + line.length) <= maxChars) {
          lineBuffer += `\n${line}`;
        } else {
          chunks.push(lineBuffer);
          lineBuffer = line;
        }
      }
      if (lineBuffer) {
        chunks.push(lineBuffer);
      }
      continue;
    }

    if (!current) {
      current = paragraph;
      continue;
    }

    if ((current.length + 2 + paragraph.length) <= maxChars) {
      current += `\n\n${paragraph}`;
    } else {
      pushCurrent();
      current = paragraph;
    }
  }

  pushCurrent();
  return chunks;
}

async function translateTextOnce(text, targetLocale) {
  const urlGuard = protectCommonURLs(text);
  const codeGuard = protectInlineCode(urlGuard.text);
  const body = new URLSearchParams({
    q: codeGuard.text,
    langpair: `en|${targetLocale === 'zh' ? 'zh-CN' : targetLocale}`,
  });
  let attempt = 0;
  while (true) {
    const response = await fetch(`${MYMEMORY_ENDPOINT}?${body.toString()}`);
    if (response.ok) {
      const payload = await response.json();
      const translated = payload?.responseData?.translatedText;
      if (typeof translated !== 'string') {
        throw new Error(`translate failed for ${targetLocale}: missing translatedText`);
      }
      const restored = urlGuard.restore(codeGuard.restore(translated));
      await sleep(4000);
      return restored;
    }
    if (response.status !== 429 && response.status < 500) {
      throw new Error(`translate failed for ${targetLocale}: ${response.status} ${response.statusText}`);
    }
    attempt += 1;
    if (attempt > 8) {
      throw new Error(`translate failed for ${targetLocale}: ${response.status} ${response.statusText}`);
    }
    const delay = Math.min(180000, 15000 * (2 ** (attempt - 1)));
    process.stderr.write(`rate limited for ${targetLocale}, retrying in ${delay}ms\n`);
    await sleep(delay);
  }
}

async function translateText(text, targetLocale) {
  const chunks = splitParagraphs(text);
  const translatedChunks = [];
  for (const chunk of chunks) {
    translatedChunks.push(await translateTextOnce(chunk, targetLocale));
  }
  return translatedChunks.join('\n\n');
}

async function translateMarkdown(markdown, targetLocale) {
  const chunks = splitMarkdown(markdown);
  const translated = [];
  for (const chunk of chunks) {
    if (chunk.type === 'code') {
      translated.push(chunk.value);
      continue;
    }
    if (!chunk.value.trim()) {
      translated.push(chunk.value);
      continue;
    }
    translated.push(await translateText(chunk.value, targetLocale));
  }
  return translated.join('\n');
}

async function writeTranslatedFile(sourceFile, targetLocale) {
  const relativePath = path.relative(DOCS_ROOT, sourceFile);
  const targetFile = path.join(LOCALES_ROOT, targetLocale, relativePath);
  const source = await fs.readFile(sourceFile, 'utf8');
  const translated = await translateMarkdown(source, targetLocale);
  const header = `> Locale: ${targetLocale} · ${localeLabel(targetLocale)}\n\n`;
  await fs.mkdir(path.dirname(targetFile), { recursive: true });
  await fs.writeFile(targetFile, header + translated, 'utf8');
  return targetFile;
}

async function main() {
  const args = process.argv.slice(2);
  if (args.length === 0) {
    usage();
    process.exitCode = 1;
    return;
  }

  const localeArg = args[0];
  const targetLocales = localeArg === 'all' ? Object.keys(LOCALES) : [localeArg];
  for (const locale of targetLocales) {
    if (!LOCALES[locale]) {
      throw new Error(`unknown locale: ${locale}`);
    }
  }

  const requestedDocs = args.slice(1).map((doc) => doc.replace(/^docs\//, ''));
  const sourceFiles = requestedDocs.length > 0
    ? requestedDocs.map((doc) => path.join(DOCS_ROOT, doc))
    : await walkMarkdownFiles(DOCS_ROOT);

  for (const sourceFile of sourceFiles) {
    const relativePath = path.relative(DOCS_ROOT, sourceFile);
    if (relativePath.startsWith(`locales${path.sep}`)) {
      continue;
    }
    for (const locale of targetLocales) {
      process.stderr.write(`Translating ${relativePath} -> ${locale}\n`);
      await writeTranslatedFile(sourceFile, locale);
    }
  }
}

main().catch((error) => {
  console.error(error.stack || String(error));
  process.exitCode = 1;
});
