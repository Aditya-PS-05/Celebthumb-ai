import { SSTConfig } from "sst";
import { APIStack } from "./infrastructure/stacks/api";

export default {
  config(_input) {
    return {
      name: "celebthumb-ai",
      region: "us-east-1",
    };
  },
  stacks(app) {
    app.stack(APIStack);
  }
} satisfies SSTConfig;