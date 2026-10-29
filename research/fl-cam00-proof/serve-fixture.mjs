import { createServer } from 'node:http';
import { readFile } from 'node:fs/promises';

const resources = new Map([
  ['/', [new URL('./fixture-viewer.html', import.meta.url), 'text/html; charset=utf-8']],
  ['/mannequin.jpg', [new URL('./assets/dummy/mannequin.jpg', import.meta.url), 'image/jpeg']],
]);
createServer(async (request, response) => {
  const resource = resources.get(request.url);
  if (request.method !== 'GET' || !resource) {
    response.writeHead(404).end();
    return;
  }
  try {
    const data = await readFile(resource[0]);
    response.writeHead(200, { 'Content-Type': resource[1], 'Cache-Control': 'no-store' });
    response.end(data);
  } catch {
    response.writeHead(404).end('Local mannequin fixture unavailable');
  }
}).listen(8769, '127.0.0.1', () => console.log('CAM01 fixture: http://127.0.0.1:8769'));
