const WebSocket = require('ws');
const net = require('net');

const WS_PORT = 7001;
const TCP_HOST = 'host.docker.internal';
// const TCP_HOST = 'node-gate-1';
const TCP_PORT = 17001;

// 协议常量
const HEAD_LEN = 12;

const wss = new WebSocket.Server({
    port: WS_PORT,
    maxPayload: 10000 * 1024 * 1024,
    perMessageDeflate: false
});

console.log(`桥接器启动成功，端口 ${WS_PORT}`);

wss.on('connection', (ws, req) => {
    const clientIp = req.socket.remoteAddress;
    console.log(`[${new Date().toISOString()}] WS连接: ${clientIp}`);

    const tcp = net.createConnection({
        host: TCP_HOST,
        port: TCP_PORT,
        noDelay: true
    });

    let isClosed = false;

    // ====== TCP 粘包处理缓存 ======
    let recvBuffer = Buffer.alloc(0);

    // ====== 清理 ======
    function cleanup() {
        if (isClosed) return;
        isClosed = true;

        recvBuffer = Buffer.alloc(0);

        try { tcp.destroy(); } catch {}
        try { ws.close(); } catch {}
    }

    // ====== WS -> TCP ======
    ws.on('message', (data) => {
        if (isClosed) return;

        const buf = Buffer.isBuffer(data) ? data : Buffer.from(data);

        console.log(`[${new Date().toISOString()}] WS -> TCP, ${buf.length} bytes`);

        // 直接写（不做确认机制）
        const ok = tcp.write(buf);

        // backpressure（很关键）
        if (!ok) {
            console.warn(`[${new Date().toISOString()}] TCP写缓冲满，暂停WS读取`);
            ws._socket.pause();

            tcp.once('drain', () => {
                console.log(`[${new Date().toISOString()}] TCP恢复写入`);
                ws._socket.resume();
            });
        }
    });

    // ====== TCP -> WS（带拆包） ======
    tcp.on('data', (data) => {
        if (isClosed) return;

        console.log(`[${new Date().toISOString()}] TCP 收到 ${data.length} bytes`);

        // 1. 拼接缓存
        recvBuffer = Buffer.concat([recvBuffer, data]);

        // 2. 循环拆包
        while (true) {
            // 头不够
            if (recvBuffer.length < HEAD_LEN) {
                return;
            }

            // 解析头
            const bodyLen = recvBuffer.readUInt16BE(0);
            const totalLen = HEAD_LEN + bodyLen;

            // 包不完整
            if (recvBuffer.length < totalLen) {
                return;
            }

            // 取完整包
            const packet = recvBuffer.slice(0, totalLen);

            // 剩余数据
            recvBuffer = recvBuffer.slice(totalLen);

            // 打印部分信息（可选）
            const protocol = packet.readUInt16BE(10);
            console.log(`[${new Date().toISOString()}] 完整包: len=${totalLen}, protocol=${protocol}`);

            // 转发给 WS（按消息边界发送）
            if (ws.readyState === WebSocket.OPEN) {
                ws.send(packet);
            }
        }
    });

    tcp.on('connect', () => {
        console.log(`[${new Date().toISOString()}] TCP连接成功 ${TCP_HOST}:${TCP_PORT}`);
    });

    tcp.on('error', (err) => {
        console.error(`[${new Date().toISOString()}] TCP错误:`, err.message);
        cleanup();
    });

    tcp.on('close', () => {
        console.log(`[${new Date().toISOString()}] TCP关闭`);
        cleanup();
    });

    ws.on('close', () => {
        console.log(`[${new Date().toISOString()}] WS断开: ${clientIp}`);
        cleanup();
    });

    ws.on('error', (err) => {
        console.error(`[${new Date().toISOString()}] WS错误:`, err.message);
        cleanup();
    });
});