const fs = require('fs');
const path = require('path');
const { Resvg } = require('@resvg/resvg-js');

const svgPath = path.join(__dirname, '..', 'docs', 'images', 'logo.svg');
const pngPath = path.join(__dirname, '..', 'docs', 'images', 'logo.png');

const svg = fs.readFileSync(svgPath, 'utf8');
const resvg = new Resvg(svg, {
  fitTo: { mode: 'width', value: 1024 }
});
const pngData = resvg.render();
const pngBuffer = pngData.asPng();

fs.writeFileSync(pngPath, pngBuffer);
console.log('Successfully rendered logo.png via Resvg! Size:', pngBuffer.length, 'bytes');
