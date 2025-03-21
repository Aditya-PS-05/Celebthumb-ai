import { StackContext, Cognito, use } from "sst/constructs";
import { APIStack } from "./api";

export function AuthStack({ stack }: StackContext) {
  // Create a Cognito User Pool
  const auth = new Cognito(stack, "Auth", {
    login: ["email", "username"],
    cdk: {
      userPool: {
        selfSignUpEnabled: true,
        autoVerify: {
          email: true,
        },
        passwordPolicy: {
          minLength: 8,
          requireLowercase: true,
          requireUppercase: true,
          requireDigits: true,
          requireSymbols: false,
        },
      },
    },
  });

  // Output the User Pool ID and Client ID
  stack.addOutputs({
    UserPoolId: auth.userPoolId,
    UserPoolClientId: auth.userPoolClientId,
  });

  return {
    auth,
  };
}