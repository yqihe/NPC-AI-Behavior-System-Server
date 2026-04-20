// WebSocket 压测脚本（k6）
// 详见 test/benchmark/README.md
//
// 场景：
// 1. 建立 WS 连接
// 2. spawn N 个 NPC（civilian 与 police 各半）
// 3. 持续发送 publish_event，测量到下一次 world_snapshot 的端到端延迟
// 4. 持续 DURATION 时间后，清理 NPC 并断开

import ws from 'k6/ws';
import { check } from 'k6';
import { Trend, Counter } from 'k6/metrics';

const NPC_COUNT = parseInt(__ENV.NPC_COUNT || '100');
const DURATION = __ENV.DURATION || '60s';
const EVENT_RPS = parseInt(__ENV.EVENT_RPS || '1');
const WS_URL = __ENV.WS_URL || 'ws://localhost:9820/ws';

const spawnLatency = new Trend('npc_spawn_latency', true);
const eventLatency = new Trend('event_to_snapshot_latency', true);
const snapshotInterval = new Trend('snapshot_interval', true);
const wsErrors = new Counter('ws_errors');

export const options = {
    vus: 1,
    duration: DURATION,
    thresholds: {
        ws_errors: ['count<10'],
        npc_spawn_latency: ['p(99)<500'],
        event_to_snapshot_latency: ['p(99)<300'],
    },
};

function nowMs() {
    return Date.now();
}

function genId(prefix) {
    return `${prefix}_${Math.random().toString(36).slice(2, 10)}`;
}

export default function () {
    const res = ws.connect(WS_URL, {}, function (socket) {
        const pendingSpawns = new Map();
        const pendingEvents = [];
        let lastSnapshotAt = 0;
        const spawnedNpcIds = [];

        socket.on('open', () => {
            for (let i = 0; i < NPC_COUNT; i++) {
                const reqId = genId('spawn');
                const npcId = `npc_${i}`;
                const typeName = i % 2 === 0 ? 'civilian' : 'police';
                pendingSpawns.set(reqId, nowMs());
                socket.send(JSON.stringify({
                    type: 'spawn_npc',
                    id: reqId,
                    data: {
                        npc_id: npcId,
                        type_name: typeName,
                        x: Math.random() * 1000,
                        z: Math.random() * 1000,
                    },
                }));
                spawnedNpcIds.push(npcId);
            }
        });

        socket.on('message', (raw) => {
            let msg;
            try {
                msg = JSON.parse(raw);
            } catch (e) {
                wsErrors.add(1);
                return;
            }

            if (msg.type === 'response' && pendingSpawns.has(msg.id)) {
                spawnLatency.add(nowMs() - pendingSpawns.get(msg.id));
                pendingSpawns.delete(msg.id);
                return;
            }

            if (msg.type === 'error') {
                wsErrors.add(1);
                console.warn(`ws error: ${msg.data && msg.data.code} ${msg.data && msg.data.message}`);
                return;
            }

            if (msg.type === 'world_snapshot') {
                const t = nowMs();
                if (lastSnapshotAt > 0) {
                    snapshotInterval.add(t - lastSnapshotAt);
                }
                lastSnapshotAt = t;

                while (pendingEvents.length > 0) {
                    const ev = pendingEvents.shift();
                    eventLatency.add(t - ev.at);
                }
                return;
            }
        });

        socket.on('error', (e) => {
            wsErrors.add(1);
            console.error(`ws socket error: ${e.error()}`);
        });

        const eventIntervalMs = Math.max(50, Math.floor(1000 / EVENT_RPS));
        socket.setInterval(() => {
            const reqId = genId('evt');
            const eventTypes = ['explosion', 'gunshot', 'shout', 'fire'];
            const evType = eventTypes[Math.floor(Math.random() * eventTypes.length)];
            pendingEvents.push({ at: nowMs() });
            socket.send(JSON.stringify({
                type: 'publish_event',
                id: reqId,
                data: {
                    event_type: evType,
                    x: Math.random() * 1000,
                    z: Math.random() * 1000,
                    source_id: 'bench',
                },
            }));
        }, eventIntervalMs);

        socket.setTimeout(() => {
            for (const npcId of spawnedNpcIds) {
                socket.send(JSON.stringify({
                    type: 'remove_npc',
                    id: genId('rm'),
                    data: { npc_id: npcId },
                }));
            }
            socket.close();
        }, parseDurationMs(DURATION) - 2000);
    });

    check(res, { 'ws status is 101': (r) => r && r.status === 101 });
}

function parseDurationMs(d) {
    const m = /^(\d+)(ms|s|m|h)$/.exec(d);
    if (!m) return 60000;
    const n = parseInt(m[1]);
    switch (m[2]) {
        case 'ms': return n;
        case 's': return n * 1000;
        case 'm': return n * 60 * 1000;
        case 'h': return n * 3600 * 1000;
        default: return 60000;
    }
}
