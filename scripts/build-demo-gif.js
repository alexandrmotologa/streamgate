const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

const framesDir = 'B:/workgit/streamgate/docs/frames';
const outputGif = 'B:/workgit/streamgate/docs/images/streamgate_demo.gif';
const palette = 'B:/workgit/streamgate/docs/frames/palette.png';

// Define sequence and frame holds (in duplicate count)
// Each duplicate at 2 fps = 0.5s hold, 3 duplicates = 1.5s hold, etc.
const sequence = [
  { file: 'frame_01.png', count: 4 }, // Overview (2s)
  { file: 'frame_08.png', count: 4 }, // Channels Explorer (2s)
  { file: 'frame_09.png', count: 4 }, // Presence modal (2s)
  { file: 'frame_04.png', count: 4 }, // Test Publisher with broadcast (2s)
  { file: 'frame_05.png', count: 4 }, // Live Stream stream arriving (2s)
  { file: 'frame_06.png', count: 4 }, // Event payload inspector (2s)
  { file: 'frame_07.png', count: 4 }, // Replay verifier (2s)
];

// Clean seq_*.png
fs.readdirSync(framesDir).forEach(f => {
  if (f.startsWith('seq_')) fs.unlinkSync(path.join(framesDir, f));
});

let idx = 1;
for (const step of sequence) {
  const src = path.join(framesDir, step.file);
  if (!fs.existsSync(src)) {
    console.warn('Missing frame:', step.file);
    continue;
  }
  for (let c = 0; c < step.count; c++) {
    const dst = path.join(framesDir, `seq_${String(idx).padStart(4, '0')}.png`);
    fs.copyFileSync(src, dst);
    idx++;
  }
}

console.log(`Generated ${idx - 1} sequence frames.`);

// 1. Generate optimal palette (scale down to 960 width for responsive markdown & fast loading)
const scale = 'scale=960:-1:flags=lanczos';
console.log('Generating palette...');
execSync(`ffmpeg -y -framerate 2 -i "${framesDir}/seq_%04d.png" -vf "${scale},palettegen=max_colors=128:reserve_transparent=0" "${palette}"`, { stdio: 'inherit' });

// 2. Generate optimized GIF using paletteuse
console.log('Generating GIF...');
execSync(`ffmpeg -y -framerate 2 -i "${framesDir}/seq_%04d.png" -i "${palette}" -lavfi "${scale} [x]; [x][1:v] paletteuse=dither=bayer:bayer_scale=3" "${outputGif}"`, { stdio: 'inherit' });

const stats = fs.statSync(outputGif);
console.log(`Successfully built ${outputGif}: ${(stats.size / 1024 / 1024).toFixed(2)} MB`);
