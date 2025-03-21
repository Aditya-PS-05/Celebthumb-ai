import { StackContext, Api, Function, use } from "sst/constructs";
import { AuthStack } from "./auth";

export function APIStack({ stack }: StackContext) {
  // Reference the Auth stack
  const { auth } = use(AuthStack);

  // Create the Lambda function
  const apiFunction = new Function(stack, "APIFunction", {
    handler: "cmd/api/main.go",
    runtime: "go1.x",
    environment: {
      STAGE: stack.stage,
      AWS_REGION: stack.region,
      USER_POOL_ID: auth.userPoolId,
      USER_POOL_CLIENT_ID: auth.userPoolClientId,
      THUMBNAIL_BUCKET: stack.stage + "-thumbnails-bucket",
      USERS_TABLE: stack.stage + "-users-table",
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
    defaults: {
      authorizer: "iam",
      function: apiFunction,
    },
    routes: {
      "GET /health": {
        authorizer: "none",
        function: apiFunction,
      },
      "POST /auth/register": {
        authorizer: "none",
        function: apiFunction,
      },
      "POST /auth/login": {
        authorizer: "none",
        function: apiFunction,
      },
      "POST /thumbnails/generate": apiFunction,
      "GET /thumbnails": apiFunction,
      "GET /thumbnails/{id}": apiFunction,
      "DELETE /thumbnails/{id}": apiFunction,
      "GET /templates": apiFunction,
      "POST /templates": apiFunction,
      "POST /subscriptions": apiFunction,
      "GET /credits": apiFunction,
    },
  });

  // Attach the auth to the API
  auth.attachPermissionsForAuthUsers(stack, [api]);

  // Grant the API Lambda permissions to access the resources
  bucket.grantReadWrite(apiFunction);
  usersTable.grantReadWriteData(apiFunction);
  thumbnailsTable.grantReadWriteData(apiFunction);

  // Add additional permissions
  api.attachPermissions([
    "dynamodb:*",
    "s3:*",
    "rekognition:*",
    "sagemaker:*",
    "cognito-idp:*",
  ]);

  // Output the API URL
  stack.addOutputs({
    ApiEndpoint: api.url,
  });
  
  return {
    api
  };
}