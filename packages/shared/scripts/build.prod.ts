import { Template } from "e2b";
import { template } from "./template.js";
import { template as genowTemplate } from "./template.genow.js";

const templates = [
  { template, alias: "base" },
  { template: genowTemplate, alias: "genow-base" },
];

async function main() {
  // default to not using cache, unless they ask for it
  const useCache = process.env.USE_CACHE === 'true';

  console.log(`cache = ${useCache ? 'enabled' : 'disabled'}`);

  for (const { template, alias } of templates) {
    console.log(`building ${alias} ...`);

    await Template.build(template, {
      alias,
      memoryMB: 512,
      skipCache: !useCache,
      onBuildLogs: (it) => console.log(it.toString()),
    });
  }
}

main().catch((err) => console.error(err));
