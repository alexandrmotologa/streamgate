const fs = require('fs');
const path = require('path');

const step = process.argv[2];
const filename = process.argv[3];

if (!step || !filename) {
  console.error('Usage: node scripts/save-frame.js <step_index> <filename>');
  process.exit(1);
}

const stepPath = `C:/Users/alexander/.gemini/antigravity/brain/141c8b2f-cdd0-4168-9859-c2af579c01fd/.system_generated/steps/${step}/output.txt`;
if (!fs.existsSync(stepPath)) {
  console.error('File not found:', stepPath);
  process.exit(1);
}

const content = fs.readFileSync(stepPath, 'utf8');
const match = content.match(/data:image\/png;base64,([A-Za-z0-9+/=]+)/) || content.match(/([A-Za-z0-9+/=]{100,})/);
if (match) {
  const target = path.join('B:/workgit/streamgate/docs/frames', filename);
  fs.writeFileSync(target, Buffer.from(match[1], 'base64'));
  console.log(`Saved ${filename} (${match[1].length} b64 chars)`);
} else {
  console.error('No base64 match in step', step);
  process.exit(1);
}
