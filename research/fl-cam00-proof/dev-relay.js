// Dev-only relay: adb reverse targets IPv4 loopback while Expo Metro bound ::1.
const net = require('node:net');

const server = net.createServer((client) => {
  const upstream = net.connect(8081, '::1');
  client.pipe(upstream).pipe(client);
  const close = () => {
    client.destroy();
    upstream.destroy();
  };
  client.on('error', close);
  upstream.on('error', close);
});

server.listen(8081, '127.0.0.1', () => {
  console.log('CAM00 relay listening on 127.0.0.1:8081 -> [::1]:8081');
});
