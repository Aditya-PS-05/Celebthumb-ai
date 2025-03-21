import { StackContext, Api, Function } from "sst/constructs";

export function APIStack({ stack }: StackContext) {
  // Create the Lambda function
  const apiFunction = new Function(stack, "APIFunction", {
    handler: "cmd/api/main.go",
    runtime: "go1.x",
    environment: {
      STAGE: stack.stage,
      AWS_REGION: stack.region,
    },
  });

  // Create the API Gateway
  const api = new Api(stack, "API", {
    cors: {
      allowMethods: ["GET", "POST", "PUT", "DELETE"],
      allowOrigins: ["*"],
      allowHeaders: [
        "Content-Type",
        "Authorization",
        "X-Api-Key",
        "X-Amz-Security-Token",
      ],
    },
    routes: {
      "GET /health": apiFunction,
      "POST /thumbnails/generate": apiFunction,
      "GET /thumbnails/{id}": apiFunction,
      "GET /templates": apiFunction,
      "POST /templates": apiFunction,
    },
  });

  // Add DynamoDB permissions
  api.attachPermissions([
    "dynamodb:*",
    "s3:*",
    "rekognition:*",
    "sagemaker:*",
  ]);

  // Output the API URL
  stack.addOutputs({
    ApiEndpoint: api.url,
  });
}