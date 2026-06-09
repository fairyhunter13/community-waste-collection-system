#!/usr/bin/env node
'use strict';

const fs = require('fs');
const path = require('path');
const { Collection } = require('postman-collection');

const PRIMARY_REQUESTS = [
  'Create Household',
  'List Households',
  'Get Household',
  'Delete Household',
  'Create Pickup (organic)',
  'Create Pickup (electronic with safety_check)',
  'List Pickups',
  'List Pickups — filter by status',
  'Schedule Pickup',
  'Complete Pickup',
  'Cancel Pickup',
  'Create Payment',
  'List Payments',
  'List Payments — filter by status',
  'Confirm Payment (multipart proof upload)',
  'Waste Summary',
  'Payment Summary',
  'Household History',
];

function fail(msg) {
  process.stderr.write('FAIL: ' + msg + '\n');
  process.exit(1);
}

const filePath = process.argv[2] ||
  path.join(__dirname, '..', 'api', 'community-waste.postman_collection.json');

let raw;
try {
  raw = JSON.parse(fs.readFileSync(filePath, 'utf8'));
} catch (e) {
  fail('could not read/parse file: ' + e.message);
}

let collection;
try {
  collection = new Collection(raw);
} catch (e) {
  fail('postman-collection SDK rejected file: ' + e.message);
}

const folderCount = collection.items.count();
if (folderCount !== 5) {
  fail('expected 5 folders, got ' + folderCount);
}

let totalRequests = 0;
let totalExamples = 0;
const requestMap = new Map();

collection.items.each(function (folder) {
  folder.items.each(function (item) {
    totalRequests++;
    const examples = item.responses ? item.responses.count() : 0;
    totalExamples += examples;
    requestMap.set(item.name, examples);
  });
});

if (totalRequests !== 27) {
  fail('expected 27 requests, got ' + totalRequests);
}

if (totalExamples < 76) {
  fail('expected >= 76 response examples, got ' + totalExamples);
}

const missing = [];
for (const name of PRIMARY_REQUESTS) {
  const count = requestMap.get(name);
  if (count === undefined) {
    missing.push(name + ' (not found)');
  } else if (count === 0) {
    missing.push(name + ' (0 examples)');
  }
}
if (missing.length > 0) {
  fail('primary requests missing examples after SDK normalization:\n  ' + missing.join('\n  '));
}

process.stdout.write(
  'OK: postman-collection SDK loaded ' + totalRequests + ' requests across ' +
  folderCount + ' folders; ' + totalExamples + ' response examples; ' +
  'all ' + PRIMARY_REQUESTS.length + ' primary requests have examples after SDK normalization\n'
);
