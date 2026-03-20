const WebSocket = require('ws');
const net = require('net');

const wss = new WebSocket.Server({
    port: 7001,//对外监听端口
    maxPayload: 10000 * 1024 * 1024,
    perMessageDeflate: false
});

wss.on('connection', (ws) => {
    const tcp = net.createConnection({
        // host: 'host.docker.internal',
        host:'node-gate-1',
        port: 17001,
        noDelay: true
    });

    // 发送队列，带确认
    let sendQueue = [];
    let currentResolve = null;
    let timeout = null;

    // 发送并等待确认（通过TCP回复确认）
    function sendAndWait(data) {
        return new Promise((resolve, reject) => {
            // 设置超时
            timeout = setTimeout(() => {
                reject(new Error('发送超时'));
            }, 5000);

            currentResolve = () => {
                clearTimeout(timeout);
                resolve();
            };

            // 发送
            tcp.write(data);
        });
    }

    // 处理队列
    async function processQueue() {
        while (sendQueue.length > 0) {
            const data = sendQueue[0];
            try {
                await sendAndWait(data);
                sendQueue.shift(); // 成功才出队
                console.log('发送成功，剩余', sendQueue.length);
            } catch (err) {
                console.error('发送失败，重试:', err);
                // 重试，不出队
                await new Promise(r => setTimeout(r, 100));
            }
        }
    }

    ws.on('message', (data) => {
        const buf = Buffer.isBuffer(data) ? data : Buffer.from(data);
        sendQueue.push(buf);
        processQueue();
    });

    // TCP回复作为确认（假设服务器会回显或回复）
    tcp.on('data', (data) => {
        // 转发给WebSocket
        ws.send(data);

        // 触发确认（简单策略：有数据回来就认为上一条成功了）
        if (currentResolve) {
            currentResolve();
            currentResolve = null;
        }
    });

    ws.on('close', () => tcp.end());
    tcp.on('end', () => ws.close());
});

console.log('确认机制桥接器启动，端口 7001');