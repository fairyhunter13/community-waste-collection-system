#!/usr/bin/env node
'use strict';

const fs = require('fs');
const path = require('path');
const importers = require('insomnia-importers');

function fail(msg) {
  process.stderr.write('FAIL: ' + msg + '\n');
  process.exit(1);
}

const filePath = process.argv[2] ||
  path.join(__dirname, '..', 'api', 'community-waste.insomnia_collection.json');

let raw;
try {
  raw = fs.readFileSync(filePath, 'utf8');
} catch (e) {
  fail('could not read file: ' + e.message);
}

(async () => {
  let result;
  try {
    result = await importers.convert(raw);
  } catch (e) {
    fail('insomnia-importers rejected file: ' + e.message);
  }

  if (!result || !result.type || !result.data) {
    fail('importer returned unexpected shape: ' + JSON.stringify(result).slice(0, 120));
  }

  // result.type is an object { id, name, description }
  if (result.type.id !== 'insomnia-4') {
    fail('expected format id "insomnia-4", got "' + result.type.id + '"');
  }

  if (result.data.__export_format !== 4) {
    fail('expected __export_format 4, got ' + result.data.__export_format);
  }

  const resources = result.data.resources || [];

  const counts = {};
  for (const r of resources) {
    const t = r._type || '?';
    counts[t] = (counts[t] || 0) + 1;
  }

  const expected = { workspace: 1, environment: 1, request_group: 5, request: 27 };
  for (const [type, count] of Object.entries(expected)) {
    if (counts[type] !== count) {
      fail('expected ' + count + ' resources of type "' + type + '", got ' + (counts[type] || 0));
    }
  }

  const requests = resources.filter(r => r._type === 'request');
  const emptyDesc = requests.filter(r => !r.description || !r.description.trim());
  if (emptyDesc.length > 0) {
    fail(
      emptyDesc.length + ' requests have empty description after import: ' +
      emptyDesc.slice(0, 3).map(r => r.name).join(', ')
    );
  }

  const env = resources.find(r => r._type === 'environment');
  if (!env) {
    fail('no environment resource found');
  }
  if (!('fixtures_dir' in (env.data || {}))) {
    fail('environment does not define "fixtures_dir" variable');
  }

  process.stdout.write(
    'OK: insomnia-importers parsed format "' + result.type.id + '" (__export_format=' +
    result.data.__export_format + ') — ' +
    counts.workspace + ' workspace, ' + counts.environment + ' environment, ' +
    counts.request_group + ' request_groups, ' + counts.request + ' requests; ' +
    'all descriptions non-empty; fixtures_dir env var defined\n'
  );
})();
