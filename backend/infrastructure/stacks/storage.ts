import { StackContext, Bucket, Table } from "sst/constructs";

export function StorageStack({ stack }: StackContext) {
  // Create an S3 bucket for storing thumbnails
  const bucket = new Bucket(stack, "ThumbnailsBucket", {
    cors: [
      {
        allowedMethods: ["GET", "PUT", "POST", "DELETE", "HEAD"],
        allowedOrigins: ["*"],
        allowedHeaders: ["*"],
      },
    ],
  });

  // Create a DynamoDB table for users
  const usersTable = new Table(stack, "UsersTable", {
    fields: {
      id: "string",
      email: "string",
      plan: "string",
      credits: "number",
      createdAt: "string",
    },
    primaryIndex: { partitionKey: "id" },
    globalIndexes: {
      byEmail: { partitionKey: "email" },
    },
  });

  // Create a DynamoDB table for thumbnails
  const thumbnailsTable = new Table(stack, "ThumbnailsTable", {
    fields: {
      id: "string",
      userId: "string",
      videoTitle: "string",
      description: "string",
      style: "string",
      url: "string",
      createdAt: "string",
    },
    primaryIndex: { partitionKey: "id" },
    globalIndexes: {
      byUser: { partitionKey: "userId" },
    },
  });

  return {
    bucket,
    usersTable,
    thumbnailsTable,
  };
}