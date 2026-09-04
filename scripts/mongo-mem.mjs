import { MongoMemoryServer } from "mongodb-memory-server";

const mongod = await MongoMemoryServer.create({
  instance: { port: 27017, dbName: "azula" },
});
console.log("mongodb-memory-server listening", mongod.getUri());
process.on("SIGINT", async () => {
  await mongod.stop();
  process.exit(0);
});
await new Promise(() => {});
