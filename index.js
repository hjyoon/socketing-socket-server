import Fastify from "fastify";
import { WebSocket, WebSocketServer } from "ws";
import jwt from "jsonwebtoken";
import fastifyEnv from "@fastify/env";
import cors from "@fastify/cors";
import fastifyRedis from "@fastify/redis";
import fastifyPostgres from "@fastify/postgres";
// import fastifyRabbit from "fastify-rabbitmq";
import crypto from "node:crypto";

const SELECTION_TIMEOUT = 10 * 1000; // 선택 만료 시간: 10초
const PAYMENT_TIMEOUT = 60 * 1000; // 결제 만료 시간: 1분 (테스트용)
const RESERVATION_BROADCAST_CHANNEL = "socketing:reservation:broadcast";

const schema = {
  type: "object",
  required: [
    "PORT",
    "JWT_SECRET",
    "JWT_SECRET_FOR_ENTRANCE",
    "CACHE_HOST",
    "CACHE_PORT",
    "DB_URL",
    // "MQ_URL",
    "SCHEDULING_SERVER_URL",
  ],
  properties: {
    PORT: {
      type: "string",
    },
    JWT_SECRET: {
      type: "string",
    },
    JWT_SECRET_FOR_ENTRANCE: {
      type: "string",
    },
    CACHE_HOST: {
      type: "string",
    },
    CACHE_PORT: {
      type: "integer",
    },
    DB_URL: {
      type: "string",
    },
    // MQ_URL: {
    //   type: "string",
    // },
    SCHEDULING_SERVER_URL: {
      type: "string",
    },
  },
};

const createServiceUrl = (baseUrl, path) =>
  new URL(path, baseUrl.endsWith("/") ? baseUrl : `${baseUrl}/`).toString();

const fastify = Fastify({
  logger: true,
});

await fastify.register(fastifyEnv, {
  schema,
  dotenv: true,
});

await fastify.register(cors, {
  origin: "*",
});

await fastify.register(fastifyRedis, {
  host: fastify.config.CACHE_HOST,
  port: fastify.config.CACHE_PORT,
  family: 4,
});

await fastify.register(fastifyPostgres, {
  connectionString: fastify.config.DB_URL,
});

// await fastify.register(fastifyRabbit, {
//   connection: fastify.config.MQ_URL,
// });

fastify.get("/liveness", (request, reply) => {
  reply.send({ status: "ok", message: "The server is alive." });
});

fastify.get("/readiness", async (request, reply) => {
  try {
    let redisStatus = { status: "disconnected", message: "" };
    let dbStatus = { status: "disconnected", message: "" };
    // let rabbitStatus = { status: "disconnected", message: "" };

    // Redis 상태 확인
    try {
      const pingResult = await fastify.redis.ping();
      if (pingResult === "PONG") {
        redisStatus = { status: "connected", message: "Redis is available." };
      } else {
        redisStatus.message = "Redis responded, but not with 'PONG'.";
      }
    } catch (error) {
      redisStatus.message = `Redis connection failed: ${error.message}`;
    }

    // PostgreSQL 상태 확인
    let client;
    try {
      client = await fastify.pg.connect();
      if (client) {
        dbStatus = {
          status: "connected",
          message: "PostgreSQL is connected and responsive.",
        };
        client.release(); // 연결 반환
      }
    } catch (error) {
      dbStatus.message = `PostgreSQL connection failed: ${error.message}`;
    }

    // RabbitMQ 상태 확인
    // try {
    //   if (fastify.rabbitmq.ready) {
    //     rabbitStatus = {
    //       status: "connected",
    //       message: "RabbitMQ is connected and operational.",
    //     };
    //   } else {
    //     rabbitStatus.message = "RabbitMQ is not connected.";
    //   }
    // } catch (error) {
    //   rabbitStatus.message = `RabbitMQ connection check failed: ${error.message}`;
    // }

    // 모든 상태가 정상일 때
    if (
      redisStatus.status === "connected" &&
      dbStatus.status === "connected"
      // rabbitStatus.status === "connected"
    ) {
      reply.send({
        status: "ok",
        message: "The server is ready.",
        redis: redisStatus,
        database: dbStatus,
        // rabbitmq: rabbitStatus,
      });
    } else {
      // 하나라도 비정상일 때
      reply.status(500).send({
        status: "error",
        message: "The server is not fully ready. See details below.",
        redis: redisStatus,
        database: dbStatus,
        // rabbitmq: rabbitStatus,
      });
    }
  } catch (unexpectedError) {
    // 예기치 못한 오류 처리
    fastify.log.error(
      "Readiness check encountered an unexpected error:",
      unexpectedError
    );
    reply.status(500).send({
      status: "error",
      message: "Unexpected error occurred during readiness check.",
      error: unexpectedError.message,
    });
  }
});

// 이벤트에 대한 모든 구역 정보를 가져오는 함수
async function getAreasForRoom(eventId) {
  // PostgreSQL 쿼리 실행
  const query = `
    SELECT
      area.id,
      area.label,
      area.svg,
      area.price
    FROM area
    WHERE area."eventId" = $1
      AND area."deletedAt" IS NULL;
  `;
  const params = [eventId];

  const { rows } = await fastify.pg.query(query, params);

  // 구역 정보를 반환
  return rows;
}

async function getSeatsForArea(eventDateId, areaId) {
  // PostgreSQL 쿼리 실행
  const query = `
    SELECT
      seat.id AS seat_id,
      seat.cx,
      seat.cy,
      seat.row,
      seat.number,
      seat."areaId" AS area_id,
      reservation.id AS reservation_id,
      eventDate.id AS event_date_id,
      eventDate.date,
      "order"."userId" AS reserved_user_id
    FROM seat
    LEFT JOIN reservation ON reservation."seatId" = seat.id AND reservation."canceledAt" IS NULL AND reservation."deletedAt" IS NULL
    LEFT JOIN event_date AS eventDate ON reservation."eventDateId" = eventDate.id
    LEFT JOIN "order" ON reservation."orderId" = "order".id AND "order"."canceledAt" IS NULL AND "order"."deletedAt" IS NULL
    WHERE seat."areaId" = $1
      AND (eventDate.id = $2 OR eventDate.id IS NULL);
  `;
  const params = [areaId, eventDateId];

  const { rows } = await fastify.pg.query(query, params);

  // 데이터 가공
  const seatMap = new Map();

  rows.forEach((row) => {
    if (!seatMap.has(row.seat_id)) {
      seatMap.set(row.seat_id, {
        id: row.seat_id,
        cx: row.cx,
        cy: row.cy,
        row: row.row,
        number: row.number,
        area_id: row.area_id,
        selectedBy: null,
        reservedUserId: row.reserved_user_id || null, // 예약된 유저 ID
        updatedAt: null, // 초기 상태
        expirationTime: null, // 초기 상태
      });
    }
  });

  return Array.from(seatMap.values());
}

// Redis에서 구역 정보를 저장
// async function setAreaDataInRedis(roomName, areaData) {
//   await fastify.redis.set(`areaData:${roomName}`, JSON.stringify(areaData));
// }

// 구역 별 예약 상태를 Redis에 저장
async function updateAreaInRedis(roomName, areaId, area) {
  await fastify.redis.hset(`areas:${roomName}`, areaId, JSON.stringify(area));
}

// Redis에서 구역 별 예약 상태 가져오기
async function getAreaFromRedis(roomName, areaId) {
  const areaData = await fastify.redis.hget(`areas:${roomName}`, areaId);
  return areaData ? JSON.parse(areaData) : null;
}

// Redis에서 모든 구역 가져오기
async function getAllAreasFromRedis(roomName) {
  const areasData = await fastify.redis.hgetall(`areas:${roomName}`);
  const areas = [];
  for (const areaId in areasData) {
    areas.push(JSON.parse(areasData[areaId]));
  }
  return areas;
}

// Redis에서 좌석 정보를 구역 별로 저장
// async function setSeatDataInRedis(areaName, seatData) {
//   await fastify.redis.set(`seatData:${areaName}`, JSON.stringify(seatData));
// }

// 좌석 선택 상태를 Redis에 저장
async function updateSeatInRedis(areaName, seatId, seat) {
  await fastify.redis.hset(`seats:${areaName}`, seatId, JSON.stringify(seat));
}

// Redis에서 좌석 선택 상태 가져오기
async function getSeatFromRedis(areaName, seatId) {
  const seatData = await fastify.redis.hget(`seats:${areaName}`, seatId);
  return seatData ? JSON.parse(seatData) : null;
}

// Redis에서 특정 구역의 모든 좌석 가져오기
async function getAllSeatsFromRedis(areaName) {
  const seatsData = await fastify.redis.hgetall(`seats:${areaName}`);
  const seats = [];
  for (const seatId in seatsData) {
    seats.push(JSON.parse(seatsData[seatId]));
  }
  return seats;
}

// 좌석 선택 만료를 Redis에서 설정
async function setSeatExpirationInRedis(areaName, seatId) {
  // 만료 시간을 설정하여 키를 설정
  await fastify.redis.set(
    `timer:${areaName}:${seatId}`,
    "active",
    "PX",
    SELECTION_TIMEOUT
  );
}

// Redis에서 좌석 선택 만료 확인
async function isSeatExpired(areaName, seatId) {
  const status = await fastify.redis.exists(`timer:${areaName}:${seatId}`);
  return !status; // 존재하지 않으면 만료됨
}

// 새로운 Order를 Redis에 임시 저장
async function createOrderInRedis(areaName, seatIds, userId, eventDateId) {
  let id = crypto.randomUUID();
  const orderStatus = "pending"; // 초기 상태
  const createdAt = new Date().toISOString();

  // Redis에 Order 데이터 저장
  await fastify.redis.hset(
    `order:${areaName}`,
    id,
    JSON.stringify({ userId, eventDateId, seatIds, orderStatus, createdAt })
  );
  return id;
}

async function updateOrderInRedis(areaName, orderId) {
  try {
    // Redis에서 주문 데이터를 가져옴
    const orderData = await getOrderFromRedis(areaName, orderId);

    // 주문 데이터가 존재하지 않는 경우 예외 처리
    if (!orderData) {
      throw new Error(
        `Order not found in Redis for area: ${areaName}, orderId: ${orderId}`
      );
    }

    // 상태 업데이트
    orderData.orderStatus = "completed";

    // 업데이트된 데이터를 Redis에 저장
    await fastify.redis.hset(
      `order:${areaName}`,
      orderId,
      JSON.stringify(orderData)
    );

    // 업데이트 결과 확인
    const updatedRedisOrder = await getOrderFromRedis(areaName, orderId);
    return updatedRedisOrder;
  } catch (error) {
    console.error("Error updating order in Redis:", error);
    throw error; // 예외를 호출자로 전달
  }
}

// Redis에서 임시 주문 정보 가져오기
async function getOrderFromRedis(areaName, orderId) {
  const orderData = await fastify.redis.hget(`order:${areaName}`, orderId);
  return orderData ? JSON.parse(orderData) : null;
}

// 주문 결제 만료를 Redis에서 설정
async function setPaymentExpirationInRedis(areaName, orderId) {
  // 만료 시간을 설정하여 키를 설정
  await fastify.redis.set(
    `paymentTimer:${areaName}:${orderId}`,
    "active",
    "PX",
    PAYMENT_TIMEOUT
  );
}

// // Redis에서 주문 결제 만료 확인
// async function isPaymentExpired(areaName, orderId) {
//   const status = await fastify.redis.exists(`paymentTimer:${areaName}:${orderId}`);
//   return !status; // 존재하지 않으면 만료됨
// }

async function validateToken(token) {
  const status = await fastify.redis.get(`token:${token}`);
  if (status === "issued") {
    await fastify.redis.del(`token:${token}`); // 토큰 사용 완료 처리
    return true;
  }
  return false;
}

// Redis Keyspace Notifications를 위한 Subscriber 설정
const redisSubscriber = fastify.redis.duplicate();
// await redisSubscriber.connect();

// Redis Keyspace Notifications 설정
await redisSubscriber.config("SET", "notify-keyspace-events", "Ex");

// 만료 이벤트 패턴 구독
const pattern = `__keyevent@${fastify.redis.options.db || 0}__:expired`;

redisSubscriber.psubscribe(pattern, (err, count) => {
  if (err) {
    fastify.log.error("Failed to subscribe to pattern:", err);
  } else {
    fastify.log.info(
      `Successfully subscribed to pattern: ${pattern}, subscription count: ${count}`
    );
  }
});

// 패턴 메시지 이벤트 리스너 설정
redisSubscriber.on("pmessage", async (pattern, channel, message) => {
  const keyParts = message.split(":");
  const keyType = keyParts[0];
  if (keyType === "timer") {
    const areaName = keyParts[1];
    const seatId = keyParts[2];

    await handleExpirationEvent(areaName, seatId);
  } else if (keyType === "paymentTimer") {
    const areaName = keyParts[1];
    const orderId = keyParts[2];

    const orderData = await getOrderFromRedis(areaName, orderId);
    if (orderData && orderData.orderStatus === "pending") {
      for (const seatId of orderData.seatIds) {
        await handleExpirationEvent(areaName, seatId);
      }
    }
    await fastify.redis.hdel(`order:${areaName}`, orderId);
  }
});

// Redis 잠금을 사용하여 이벤트 중복 방지
const handleExpirationEvent = async (areaName, seatId) => {
  const lockKey = `lock:seat:${areaName}:${seatId}`;

  // 잠금을 설정하고 기존에 잠금이 없었을 경우에만 처리
  const lockAcquired = await fastify.redis.set(
    lockKey,
    "locked",
    "NX",
    "EX",
    10
  );
  if (!lockAcquired) {
    fastify.log.info(`Another process is already handling this: ${lockKey}`);
    return; // 다른 프로세스가 이미 처리 중
  }

  try {
    // 좌석 정보 처리 로직
    const seat = await getSeatFromRedis(areaName, seatId);
    if (seat) {
      seat.selectedBy = null;
      seat.updatedAt = new Date().toISOString();
      seat.expirationTime = null;
      seat.reservedUserId = null;

      await updateSeatInRedis(areaName, seatId, seat);

      await publishReservationMessage({
        room: areaName,
        type: "seatsSelected",
        payload: [
          {
            seatId: seat.id,
            selectedBy: null,
            updatedAt: seat.updatedAt,
            expirationTime: null,
            reservedUserId: null,
          },
        ],
      });

      fastify.log.info(
        `Selection for seat ${seatId} has expired (area: ${areaName}).`
      );
    }
  } finally {
    // 잠금 해제
    await fastify.redis.del(lockKey);
  }
};

const CLOSE_POLICY_VIOLATION = 1008;
const wsServer = new WebSocketServer({ server: fastify.server });
const wsClients = new Map();
const wsRooms = new Map();
const reservationPubClient = fastify.redis.duplicate();
const reservationSubClient = fastify.redis.duplicate();

// Redis 기반 유저 수 가져오기 함수
async function getRoomUserCount(roomName) {
  const maxRetries = 30;
  let delay = null;
  for (let attempt = 1; attempt <= maxRetries; attempt++) {
    try {
      const count = await fastify.redis.get(`room:${roomName}:count`);
      return parseInt(count || "0"); // 소켓 수 반환
    } catch (err) {
      console.error(
        `Timeout reached, retrying (attempt ${attempt}/${maxRetries})...`
      );
      await new Promise((resolve) => {
        delay = decorrelatedJitter(100, 60000, delay);
        setTimeout(resolve, delay);
      });
    }
  }
}

async function decrementRoomCount(room) {
  const decrementScript = `
  local key = KEYS[1]
  local value = redis.call("GET", key)
  if value and tonumber(value) > 0 then
    return redis.call("DECR", key)
  else
    return 0
  end
`;
  const key = `room:${room}:count`;
  const count = await fastify.redis.eval(decrementScript, 1, key);
  return parseInt(count || "0");
}

function decorrelatedJitter(baseDelay, maxDelay, previousDelay) {
  if (!previousDelay) {
    previousDelay = baseDelay;
  }
  return Math.min(
    maxDelay,
    Math.random() * (previousDelay * 3 - baseDelay) + baseDelay
  );
}

function isRecord(value) {
  return typeof value === "object" && value !== null;
}

function sendRaw(ws, type, payload) {
  if (ws.readyState !== WebSocket.OPEN) return;
  ws.send(JSON.stringify({ type, payload }));
}

function sendMessage(client, type, payload) {
  sendRaw(client.ws, type, payload);
}

function closeClient(client, code = 1000, reason = "") {
  if (client.ws.readyState === WebSocket.OPEN) {
    client.ws.close(code, reason);
  }
}

function addClientToRoom(client, roomName) {
  if (client.rooms.has(roomName)) {
    return false;
  }

  client.rooms.add(roomName);
  if (!wsRooms.has(roomName)) {
    wsRooms.set(roomName, new Set());
  }
  wsRooms.get(roomName).add(client.id);
  return true;
}

function removeClientFromRoom(client, roomName) {
  client.rooms.delete(roomName);
  const room = wsRooms.get(roomName);
  if (!room) return;

  room.delete(client.id);
  if (room.size === 0) {
    wsRooms.delete(roomName);
  }
}

function removeClientFromAllRooms(client) {
  for (const roomName of [...client.rooms]) {
    removeClientFromRoom(client, roomName);
  }
}

function getLocalRoomClients(roomName) {
  const room = wsRooms.get(roomName);
  if (!room) return [];

  return [...room]
    .map((socketId) => wsClients.get(socketId))
    .filter((client) => client?.ws.readyState === WebSocket.OPEN);
}

function broadcastToRoom(roomName, type, payload) {
  for (const client of getLocalRoomClients(roomName)) {
    sendMessage(client, type, payload);
  }
}

function broadcastToAll(type, payload) {
  for (const client of wsClients.values()) {
    sendMessage(client, type, payload);
  }
}

function isMainRoom(roomName) {
  return roomName.split("_").length === 2;
}

function isAreaRoom(roomName) {
  return roomName.split("_").length === 3;
}

async function publishReservationMessage(message) {
  await reservationPubClient.publish(
    RESERVATION_BROADCAST_CHANNEL,
    JSON.stringify(message)
  );
}

async function handleReservationBroadcast(rawMessage) {
  let message;
  try {
    message = JSON.parse(rawMessage);
  } catch (error) {
    fastify.log.error(`Invalid reservation broadcast message: ${error.message}`);
    return;
  }

  if (!message.room || !message.type) return;
  broadcastToRoom(message.room, message.type, message.payload);
}

reservationSubClient.on("message", (channel, rawMessage) => {
  if (channel !== RESERVATION_BROADCAST_CHANNEL) return;
  void handleReservationBroadcast(rawMessage);
});

const serverTimeInterval = setInterval(() => {
  broadcastToAll("serverTime", new Date().toISOString());
}, 1000);

await reservationSubClient.subscribe(RESERVATION_BROADCAST_CHANNEL);

async function leaveMainRoom(client, roomName) {
  if (!client.rooms.has(roomName)) return;

  removeClientFromRoom(client, roomName);
  const currentConnections = await decrementRoomCount(roomName);
  fastify.log.info(
    `Client ${client.id} left room: ${roomName}. Current connections: ${currentConnections}`
  );
}

async function leaveAreaRoom(client, areaName) {
  if (!client.rooms.has(areaName)) return;

  removeClientFromRoom(client, areaName);
  const allSeats = await getAllSeatsFromRedis(areaName);
  await releaseSeats(client.id, allSeats, areaName);
  fastify.log.info(`Client ${client.id} left area: ${areaName}.`);
}

async function handleJoinRoom(client, { eventId, eventDateId } = {}) {
  if (!eventId || !eventDateId) {
    sendMessage(client, "error", { message: "Invalid room parameters." });
    return;
  }

  const roomName = `${eventId}_${eventDateId}`;

  try {
    const joined = addClientToRoom(client, roomName);
    let currentConnections = Number(
      (await fastify.redis.get(`room:${roomName}:count`)) || 0
    );
    if (joined) {
      currentConnections = await fastify.redis.incr(`room:${roomName}:count`);
    }

    fastify.log.info(
      `Client ${client.id} joined room: ${roomName}. Current connections: ${currentConnections}`
    );

    let areas = await getAllAreasFromRedis(roomName);
    if (areas.length === 0) {
      areas = await getAreasForRoom(eventId, eventDateId);
      for (const area of areas) {
        await updateAreaInRedis(roomName, area.id, area);
      }
    }

    sendMessage(client, "roomJoined", {
      message: `You have joined the room: ${roomName}`,
      areas,
    });

    const jwtToken = jwt.sign(
      {
        jti: crypto.randomUUID(),
        sub: "scheduling",
        eventId,
        eventDateId,
      },
      fastify.config.JWT_SECRET,
      {
        expiresIn: 600,
      }
    );

    await fetch(
      createServiceUrl(
        fastify.config.SCHEDULING_SERVER_URL,
        "scheduling/seat/reservation/statistic"
      ),
      {
        method: "POST",
        headers: {
          Authorization: `Bearer ${jwtToken}`,
        },
      }
    );
  } catch (error) {
    fastify.log.error(`Error fetching data for room ${roomName}:`, error);
    sendMessage(client, "error", {
      message: "Failed to fetch room data.",
    });
  }
}

async function handleJoinArea(client, { eventId, eventDateId, areaId } = {}) {
  if (!eventId || !eventDateId || !areaId) {
    sendMessage(client, "error", { message: "Invalid area parameters." });
    return;
  }

  const areaName = `${eventId}_${eventDateId}_${areaId}`;

  try {
    addClientToRoom(client, areaName);
    fastify.log.info(`Client ${client.id} joined area: ${areaName}.`);

    let seats = await getAllSeatsFromRedis(areaName);
    if (seats.length === 0) {
      seats = await getSeatsForArea(eventDateId, areaId);
      for (const seat of seats) {
        await updateSeatInRedis(areaName, seat.id, seat);
      }
    }

    sendMessage(client, "areaJoined", {
      message: `You have joined the area: ${areaName}`,
      seats,
    });
  } catch (error) {
    fastify.log.error(`Error fetching data for area ${areaName}:`, error);
    sendMessage(client, "error", {
      message: "Failed to fetch area data.",
    });
  }
}

async function handleSelectSeats(
  client,
  { seatId, eventId, eventDateId, areaId, numberOfSeats = 1 } = {}
) {
  if (!seatId || !eventId || !eventDateId || !areaId) {
    sendMessage(client, "error", { message: "Invalid selectSeats parameters." });
    return;
  }

  const areaName = `${eventId}_${eventDateId}_${areaId}`;
  const allSeats = await getAllSeatsFromRedis(areaName);

  await releaseSeats(client.id, allSeats, areaName);

  const selectedSeat = allSeats.find((s) => s.id === seatId);
  if (!selectedSeat) {
    sendMessage(client, "error", { message: "Invalid seat ID." });
    return;
  }

  const seatsToSelect = [];

  if (numberOfSeats === 1) {
    if (selectedSeat.reservedUserId) {
      sendMessage(client, "error", {
        message: `Seat ${selectedSeat.id} is reserved and cannot be selected.`,
      });
      return;
    }

    const expired = await isSeatExpired(areaName, selectedSeat.id);
    if (selectedSeat.selectedBy && !expired) {
      sendMessage(client, "error", {
        message: `Seat ${selectedSeat.id} is already selected by another user.`,
      });
      return;
    }

    seatsToSelect.push(selectedSeat);
  } else {
    const adjacentSeats = findAdjacentSeats(
      allSeats,
      selectedSeat,
      numberOfSeats
    );

    if (adjacentSeats.length < numberOfSeats) {
      sendMessage(client, "error", {
        message: "Not enough adjacent seats available",
      });
      return;
    }
    seatsToSelect.push(...adjacentSeats);
  }

  const currentTime = new Date().toISOString();
  const result = [];
  for (const seat of seatsToSelect) {
    seat.selectedBy = client.id;
    seat.updatedAt = currentTime;
    seat.expirationTime = new Date(Date.now() + SELECTION_TIMEOUT).toISOString();

    await updateSeatInRedis(areaName, seat.id, seat);
    await setSeatExpirationInRedis(areaName, seat.id);

    result.push({
      seatId: seat.id,
      selectedBy: client.id,
      updatedAt: currentTime,
      expirationTime: seat.expirationTime,
    });

    fastify.log.info(`Seat ${seat.id} selected by ${client.id}`);
  }

  await publishReservationMessage({
    room: areaName,
    type: "seatsSelected",
    payload: result,
  });
}

async function handleReserveSeats(
  client,
  { seatIds, eventId, eventDateId, areaId, userId } = {}
) {
  const roomName = `${eventId}_${eventDateId}`;
  const areaName = `${eventId}_${eventDateId}_${areaId}`;
  const seatsToReserve = [];
  const broadcastUpdates = [];
  const seatIdsToReserve = [];

  if (!eventId || !eventDateId || !areaId || !userId) {
    sendMessage(client, "error", { message: "Invalid reserveSeats parameters." });
    return;
  }

  try {
    if (!Array.isArray(seatIds) || seatIds.length === 0) {
      sendMessage(client, "error", { message: "Invalid seat IDs." });
      return;
    }

    for (const seatId of seatIds) {
      let seat = await getSeatFromRedis(areaName, seatId);
      if (!seat) {
        fastify.log.warn(`Invalid seat ID: ${seatId}`);
        sendMessage(client, "error", { message: `Invalid seat ID: ${seatId}.` });
        return;
      }

      if (seat.reservedUserId) {
        sendMessage(client, "error", {
          message: `Seat ${seat.id} is reserved and cannot be selected.`,
        });
        return;
      }

      const expired = await isSeatExpired(areaName, seat.id);
      if (seat.selectedBy !== null && seat.selectedBy !== client.id && !expired) {
        sendMessage(client, "error", {
          message: `Seat ${seat.id} is already selected by another user.`,
        });
        return;
      }

      const currentTime = new Date().toISOString();

      seat.reservedUserId = userId;
      seat.selectedBy = null;
      seat.updatedAt = currentTime;
      seat.expirationTime = null;

      await updateSeatInRedis(areaName, seatId, seat);
      fastify.log.info(
        `Seat ${seatId} will be reserved by ${client.id} in area ${areaName}`
      );

      await fastify.redis.del(`timer:${areaName}:${seatId}`);

      seatsToReserve.push(seat);
      seatIdsToReserve.push(seat.id);

      broadcastUpdates.push({
        seatId: seat.id,
        selectedBy: seat.selectedBy,
        updatedAt: seat.updatedAt,
        expirationTime: seat.expirationTime,
        reservedUserId: seat.reservedUserId,
      });
    }
  } catch (error) {
    fastify.log.error(`Failed to process seats: ${error.message}`);
    sendMessage(client, "error", {
      message: `Failed to process seats: ${error.message}`,
    });
    return;
  }

  let orderId;
  try {
    orderId = await createOrderInRedis(
      areaName,
      seatIdsToReserve,
      userId,
      eventDateId
    );
    await setPaymentExpirationInRedis(areaName, orderId);
    const order = await getOrderFromRedis(areaName, orderId);
    const selectedArea = await getAreaFromRedis(roomName, areaId);

    const area = {
      id: selectedArea.id,
      label: selectedArea.label,
      price: selectedArea.price,
    };
    const expirationTime = new Date(Date.now() + PAYMENT_TIMEOUT).toISOString();

    const reservationData = {
      id: orderId,
      createdAt: order.createdAt,
      expirationTime: expirationTime,
      seats: seatsToReserve,
      area: area,
    };

    sendMessage(client, "orderMade", { data: reservationData });
  } catch (error) {
    fastify.log.error(`Failed to prepare order data: ${error.message}`);
    sendMessage(client, "error", {
      message: `Failed to prepare order data: ${error.message}`,
    });
  }

  if (broadcastUpdates.length > 0) {
    await publishReservationMessage({
      room: areaName,
      type: "seatsSelected",
      payload: broadcastUpdates,
    });
  }
}

async function handleRequestOrder(
  wsClient,
  { userId, orderId, paymentMethod, eventId, eventDateId, areaId } = {}
) {
  if (!userId || !orderId || !paymentMethod || !eventId || !eventDateId || !areaId) {
    sendMessage(wsClient, "error", { message: "Invalid requestOrder parameters." });
    return;
  }
  const areaName = `${eventId}_${eventDateId}_${areaId}`;

  const redisOrderData = await getOrderFromRedis(areaName, orderId);
  if (!redisOrderData) {
    sendMessage(wsClient, "error", { message: "Invalid cache requestOrderData" });
    return;
  }

  const pgClient = await fastify.pg.connect();
  try {
    await pgClient.query("BEGIN");

    const userResult = await pgClient.query(`SELECT * FROM "user" WHERE id = $1`, [
      userId,
    ]);
    const user = userResult.rows[0];
    if (!user) {
      throw { code: "USER_NOT_FOUND", message: "User not found." };
    }

    const eventResult = await pgClient.query(
      `
        SELECT
          ed.id AS "eventDateId",
          ed.date AS "eventDate",
          e.id AS "eventId",
          e.title AS "eventTitle",
          e.place AS "eventPlace",
          e.cast AS "eventCast",
          e.thumbnail AS "eventThumbnail",
          e."ageLimit" AS "eventAgeLimit"
        FROM event_date ed
        INNER JOIN event e ON ed."eventId" = e.id
        WHERE ed.id = $1
      `,
      [eventDateId]
    );

    const event = eventResult.rows[0];
    if (!event) {
      throw {
        code: "EVENT_DATE_NOT_FOUND",
        message: "Event date not found.",
      };
    }

    const seatIds = redisOrderData.seatIds;
    const seatResult = await pgClient.query(
      `
        SELECT
          s.*,
          a.id AS "areaId",
          a.label AS "areaLabel",
          a.price AS "areaPrice"
        FROM seat s
        INNER JOIN area a ON s."areaId" = a.id
        WHERE s.id = ANY($1::uuid[])
      `,
      [seatIds]
    );

    const seatsArray = seatResult.rows;

    for (const seatId of seatIds) {
      const reservationCheck = await pgClient.query(
        `
          SELECT *
          FROM reservation r
          LEFT JOIN "order" o ON r."orderId" = "o".id
          WHERE "eventDateId" = $1
          AND r."seatId" = $2
          AND r."canceledAt" IS NULL
          AND r."deletedAt" IS NULL
          AND o."canceledAt" IS NULL
        `,
        [eventDateId, seatId]
      );
      if (reservationCheck.rows.length > 0) {
        throw {
          code: "EXISTING_ORDER",
          message: `Seat ${seatId} is already reserved.`,
        };
      }
    }

    const pgOrderResult = await pgClient.query(
      `
        INSERT INTO "order" ("userId", "paymentMethod")
        VALUES ($1, $2)
        RETURNING id
      `,
      [userId, paymentMethod]
    );

    const pgSavedOrder = pgOrderResult.rows[0];
    const pgSavedOrderId = pgSavedOrder.id;

    for (const seatId of seatIds) {
      await pgClient.query(
        `
          INSERT INTO reservation ("orderId", "eventDateId", "seatId")
          VALUES ($1, $2, $3)
        `,
        [pgSavedOrderId, eventDateId, seatId]
      );
    }

    const totalAmountResult = await pgClient.query(
      `
        SELECT SUM(area.price) AS "totalAmount"
        FROM reservation
        INNER JOIN seat ON reservation."seatId" = seat.id
        INNER JOIN area AS area ON seat."areaId" = area.id
        WHERE reservation."eventDateId" = $1
          AND reservation."orderId" = $2
      `,
      [eventDateId, pgSavedOrderId]
    );

    const totalAmount = Number(totalAmountResult.rows[0]?.totalAmount || 0);

    if (isNaN(totalAmount)) {
      throw { code: "INVALID_AMOUNT", message: "Invalid total amount." };
    }

    if (user.point < totalAmount) {
      throw {
        code: "INSUFFICIENT_BALANCE",
        message: "Insufficient balance.",
      };
    }

    await pgClient.query(`UPDATE "user" SET point = point - $1 WHERE id = $2`, [
      totalAmount,
      userId,
    ]);

    const responseData = {
      orderId: pgSavedOrder.id,
      orderCreatedAt: pgSavedOrder.createdAt,
      orderUpdatedAt: pgSavedOrder.updatedAt,
      orderCanceledAt: pgSavedOrder.canceledAt,
      paymentMethod: pgSavedOrder.paymentMethod,
      useId: user.id,
      userNickname: user.nickname,
      userEmail: user.email,
      userProfileImage: user.profileImage,
      userRole: user.role,
      eventId: event.eventId,
      eventTitle: event.eventTitle,
      eventPlace: event.eventPlace,
      eventCast: event.eventCast,
      eventDate: event.eventDate,
      eventThumbnail: event.eventThumbnail,
      eventAgeLimit: event.eventAgeLimit,
      reservations: seatsArray.map((seat) => ({
        seatId: seat.id,
        seatRow: seat.row,
        seatNumber: seat.number,
        seatAreaId: seat.areaId,
        seatAreaLabel: seat.areaLabel,
        seatPrice: seat.areaPrice,
      })),
    };

    await pgClient.query("COMMIT");

    await updateOrderInRedis(areaName, orderId);

    sendMessage(wsClient, "orderApproved", { success: true, data: responseData });
  } catch (error) {
    await pgClient.query("ROLLBACK");
    console.error("Error processing order request:", error);

    sendMessage(wsClient, "error", {
      error: error.code || "UNKNOWN_ERROR",
      message: error.message || "An unexpected error occurred.",
    });
  } finally {
    pgClient.release();
  }
}

async function handleExitArea(client, { eventId, eventDateId, areaId } = {}) {
  if (!eventId || !eventDateId || !areaId) {
    sendMessage(client, "error", { message: "Invalid area parameters." });
    return;
  }

  const areaName = `${eventId}_${eventDateId}_${areaId}`;

  try {
    await leaveAreaRoom(client, areaName);
    sendMessage(client, "areaExited", {
      message: `You have left the area: ${areaName}`,
    });
  } catch (error) {
    fastify.log.error(`Error exiting area ${areaName}:`, error);
    sendMessage(client, "error", {
      message: "Failed to leave current area.",
    });
  }
}

async function handleExitRoom(client, { eventId, eventDateId } = {}) {
  if (!eventId || !eventDateId) {
    sendMessage(client, "error", { message: "Invalid room parameters." });
    return;
  }

  const roomName = `${eventId}_${eventDateId}`;

  try {
    await leaveMainRoom(client, roomName);
    sendMessage(client, "roomExited", {
      message: `You have left the room: ${roomName}`,
    });
  } catch (error) {
    fastify.log.error(`Error exiting room ${roomName}:`, error);
    sendMessage(client, "error", {
      message: "Failed to leave current room.",
    });
  }
}

async function handleClientDisconnect(client) {
  if (client.disconnectHandled) return;
  client.disconnectHandled = true;

  fastify.log.info(`Client disconnected: ${client.id}`);

  const rooms = [...client.rooms].filter((roomName) => roomName !== client.id);
  for (const areaName of rooms.filter(isAreaRoom)) {
    await leaveAreaRoom(client, areaName);
  }
  for (const roomName of rooms.filter(isMainRoom)) {
    await leaveMainRoom(client, roomName);
  }

  removeClientFromAllRooms(client);
  wsClients.delete(client.id);
}

async function handleClientMessage(client, rawMessage) {
  let message;
  try {
    message = JSON.parse(rawMessage.toString());
  } catch {
    sendMessage(client, "error", { message: "Invalid message format." });
    return;
  }

  if (!isRecord(message) || typeof message.type !== "string") {
    sendMessage(client, "error", { message: "Invalid message format." });
    return;
  }

  try {
    switch (message.type) {
      case "joinRoom":
        await handleJoinRoom(client, message.payload);
        break;
      case "joinArea":
        await handleJoinArea(client, message.payload);
        break;
      case "selectSeats":
        await handleSelectSeats(client, message.payload);
        break;
      case "reserveSeats":
        await handleReserveSeats(client, message.payload);
        break;
      case "requestOrder":
        await handleRequestOrder(client, message.payload);
        break;
      case "exitArea":
        await handleExitArea(client, message.payload);
        break;
      case "exitRoom":
        await handleExitRoom(client, message.payload);
        break;
      default:
        sendMessage(client, "error", { message: "Unknown message type." });
    }
  } catch (error) {
    fastify.log.error(`Error handling ${message.type}: ${error.message}`);
    sendMessage(client, "error", { message: "Internal server error." });
  }
}

async function handleWebSocketConnection(ws, request) {
  const requestUrl = new URL(
    request.url || "/",
    `http://${request.headers.host || "localhost"}`
  );
  const token = requestUrl.searchParams.get("token");

  if (!token) {
    sendRaw(ws, "connect_error", { message: "Authentication error" });
    ws.close(CLOSE_POLICY_VIOLATION, "Authentication error");
    return;
  }

  if (!(await validateToken(token))) {
    sendRaw(ws, "connect_error", { message: "Authentication error 2" });
    ws.close(CLOSE_POLICY_VIOLATION, "Authentication error 2");
    return;
  }

  let decoded;
  try {
    decoded = jwt.verify(token, fastify.config.JWT_SECRET_FOR_ENTRANCE);
  } catch {
    sendRaw(ws, "connect_error", { message: "Authentication error" });
    ws.close(CLOSE_POLICY_VIOLATION, "Authentication error");
    return;
  }

  const client = {
    id: crypto.randomUUID(),
    ws,
    data: { user: decoded },
    rooms: new Set(),
    disconnectHandled: false,
  };

  wsClients.set(client.id, client);
  addClientToRoom(client, client.id);
  sendMessage(client, "connected", { id: client.id });

  fastify.log.info(`New WebSocket client connected: ${client.id}`);

  ws.on("message", (rawMessage) => {
    void handleClientMessage(client, rawMessage);
  });

  ws.on("close", () => {
    void handleClientDisconnect(client);
  });

  ws.on("error", (error) => {
    fastify.log.error(`WebSocket error for ${client.id}: ${error.message}`);
  });
}

wsServer.on("connection", (ws, request) => {
  void handleWebSocketConnection(ws, request);
});

async function releaseSeats(socketId, seats, areaName) {
  const currentTime = new Date().toISOString();
  const seatsToRelease = [];

  for (const seat of seats) {
    if (seat.selectedBy === socketId) {
      seat.selectedBy = null;
      seat.updatedAt = currentTime;
      seat.expirationTime = null;
      seat.reservedUserId = null;

      seatsToRelease.push({
        seatId: seat.id,
        selectedBy: seat.selectedBy,
        updatedAt: seat.updatedAt,
        expirationTime: seat.expirationTime,
        reservedUserId: seat.reservedUserId,
      });

      await fastify.redis.del(`timer:${areaName}:${seat.id}`); // Redis 만료 키 제거

      // Redis 업데이트
      await updateSeatInRedis(areaName, seat.id, seat);

      // 같은 area의 유저들에게 상태 변경 브로드캐스트

      fastify.log.info(`Seat ${seat.id} selection cancelled by ${socketId}`);
    }
  }
  if (seatsToRelease.length > 0) {
    await publishReservationMessage({
      room: areaName,
      type: "seatsSelected",
      payload: seatsToRelease,
    });
  }
}

// RabbitMQ 메시지 전송 로직
// async function sendMessageToQueue(roomName, message) {
//   const queueName = `queue:${roomName}`;
//   try {
//     // 큐 선언 (존재하지 않을 경우 생성)
//     await fastify.rabbitmq.queueDeclare({ queue: queueName, durable: true });

//     // Publisher 생성
//     const publisher = fastify.rabbitmq.createPublisher({
//       confirm: true, // 메시지가 성공적으로 전송되었는지 확인
//       maxAttempts: 3, // 최대 재시도 횟수
//     });

//     // 메시지 전송
//     await publisher.send(queueName, JSON.stringify(message));

//     fastify.log.info(`Message sent to queue "${queueName}": ${message}`);
//   } catch (error) {
//     fastify.log.error(`Failed to send message to queue "${queueName}":`, error);
//   }
// }

function findAdjacentSeats(seats, selectedSeat, numberOfSeats) {
  const selectedRow = selectedSeat.row;
  const selectedNumber = selectedSeat.number;

  // 예약되지 않은 좌석들과 선택되지 않은 좌석들만 필터링
  const availableSeats = seats.filter(
    (seat) => seat.reservedUserId === null && seat.selectedBy === null
  );

  const result = []; // 초기 배열을 비워둠

  // 중복 좌석 체크 함수
  const isSeatAlreadySelected = (seat) =>
    result.some((r) => r.row === seat.row && r.number === seat.number);

  // 초기 좌석 추가
  result.push(selectedSeat);

  let offset = 1;
  // 같은 행(row)에서 좌석 찾기
  while (result.length < numberOfSeats) {
    // 현재 offset에 따라 왼쪽과 오른쪽 좌석 번호 계산
    const positions = [
      { row: selectedRow, number: selectedNumber + offset }, // 오른쪽 좌석
      { row: selectedRow, number: selectedNumber - offset }, // 왼쪽 좌석
    ];

    let seatFound = false;

    for (const pos of positions) {
      if (result.length >= numberOfSeats) break;

      // 해당 위치에 좌석이 있는지 확인
      const seat = availableSeats.find(
        (s) =>
          s.row === pos.row && // 같은 행인지 확인
          s.number === pos.number && // 해당 좌석 번호인지 확인
          !isSeatAlreadySelected(s) // 중복 좌석 체크
      );

      if (seat) {
        result.push(seat); // 좌석을 결과에 추가
        seatFound = true;
      }
    }

    if (!seatFound) break; // 더 이상 좌석을 찾지 못하면 종료

    offset++;
  }

  // 같은 행에서 충분한 좌석을 찾지 못한 경우, 다른 행에서 좌석 찾기
  if (result.length < numberOfSeats) {
    // 동일한 구역(area) 내의 모든 행(row) 가져오기
    const rowsInArea = [...new Set(availableSeats.map((seat) => seat.row))];

    // 현재 행을 제외하고, 행 번호의 차이에 따라 가까운 순서대로 정렬
    const sortedRows = rowsInArea
      .filter((r) => r !== selectedRow)
      .sort((a, b) => Math.abs(a - selectedRow) - Math.abs(b - selectedRow));

    for (const row of sortedRows) {
      if (result.length >= numberOfSeats) break;

      offset = 0;
      while (result.length < numberOfSeats) {
        // 현재 offset에 따라 좌석 번호 계산
        const positions = [
          { row: row, number: selectedNumber + offset }, // 오른쪽 좌석
          { row: row, number: selectedNumber - offset }, // 왼쪽 좌석
        ];

        let seatFound = false;

        for (const pos of positions) {
          if (result.length >= numberOfSeats) break;

          // 해당 위치에 좌석이 있는지 확인
          const seat = availableSeats.find(
            (s) =>
              s.row === pos.row && // 해당 행인지 확인
              s.number === pos.number && // 해당 좌석 번호인지 확인
              !isSeatAlreadySelected(s) // 중복 좌석 체크
          );

          if (seat) {
            result.push(seat); // 좌석을 결과에 추가
            seatFound = true;
          }
        }

        if (!seatFound) break; // 더 이상 좌석을 찾지 못하면 종료

        offset++;
      }
    }
  }

  // 아직도 좌석을 다 찾지 못한 경우, 동일한 구역 내의 다른 좌석들을 추가
  if (result.length < numberOfSeats) {
    // 남은 좌석들을 거리 순으로 정렬
    const remainingSeats = availableSeats
      .filter((seat) => !isSeatAlreadySelected(seat)) // 이미 선택된 좌석 제외
      .sort((a, b) => {
        const rowDiff =
          Math.abs(a.row - selectedRow) - Math.abs(b.row - selectedRow);
        if (rowDiff !== 0) return rowDiff;
        return (
          Math.abs(a.number - selectedNumber) -
          Math.abs(b.number - selectedNumber)
        );
      });

    for (const seat of remainingSeats) {
      if (result.length >= numberOfSeats) break;
      result.push(seat); // 좌석 추가
    }
  }

  return result;
}

const startServer = async () => {
  try {
    const port = Number(fastify.config.PORT);
    const address = await fastify.listen({ port, host: "0.0.0.0" });

    fastify.log.info(`Server is now listening on ${address}`);

    if (process.send) {
      process.send("ready");
    }
  } catch (err) {
    fastify.log.error(err);
    process.exit(1);
  }
};

let shutdownInProgress = false; // 중복 호출 방지 플래그

async function gracefulShutdown(signal) {
  if (shutdownInProgress) {
    fastify.log.warn(
      `Shutdown already in progress. Ignoring signal: ${signal}`
    );
    return;
  }
  shutdownInProgress = true; // 중복 호출 방지

  fastify.log.info(`Received signal: ${signal}. Starting graceful shutdown...`);

  try {
    clearInterval(serverTimeInterval);

    for (const client of wsClients.values()) {
      closeClient(client, 1000, "Server shutting down");
    }
    wsServer.close();
    fastify.log.info("All WebSocket connections have been closed.");

    await redisSubscriber.disconnect();
    await reservationPubClient.disconnect();
    await reservationSubClient.disconnect();

    await fastify.close();
    fastify.log.info("Fastify server has been closed.");

    // 기타 필요한 종료 작업 (예: DB 연결 해제)
    // await database.disconnect();
    fastify.log.info("Additional cleanup tasks completed.");

    fastify.log.info("Graceful shutdown complete. Exiting process...");
    process.exit(0);
  } catch (error) {
    fastify.log.error("Error occurred during graceful shutdown:", error);
    process.exit(1);
  }
}

startServer();

process.on("SIGINT", () => gracefulShutdown("SIGINT"));
process.on("SIGTERM", () => gracefulShutdown("SIGTERM"));
