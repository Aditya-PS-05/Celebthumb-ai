import { SSTConfig } from "sst";
import { APIStack } from "./infrastructure/stacks/api";
import { AuthStack } from "./infrastructure/stacks/auth";
import { StorageStack } from "./infrastructure/stacks/storage";

export default {
  config(_input) {
    return {
      name: "celebthumb-ai",
      region: "us-east-1",
    };
  },
  stacks(app) {
    app.stack(StorageStack);
    app.stack(AuthStack);
    app.stack(APIStack);
  }
} satisfies SSTConfig;