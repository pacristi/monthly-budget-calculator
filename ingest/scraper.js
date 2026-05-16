import { getBank } from "open-banking-chile";
import * as fs from "fs";
import * as path from "path";

// En Node moderno con ESM, usamos import.meta.dirname
const currentDir = import.meta.dirname;

const RUT = process.env.BANCO_RUT;
const PASSWORD = process.env.BANCO_PASS;
const BANCO_ID = process.env.BANCO_ID || "bchile";

async function run() {
  console.log(`Iniciando ingesta de banco "${BANCO_ID}"...`);
  // Bancos soportados: ver https://www.npmjs.com/package/open-banking-chile
  const banco = getBank(BANCO_ID);


  const result = await banco.scrape({
    rut: RUT,
    password: PASSWORD,
  });

  if (!result.success) {
    console.error("Falló el scraper:", result);
    process.exit(1);
  }

  const dataStr = JSON.stringify(result, null, 2);

  // 1. Guardar en el Data Lake (Histórico) en /data/archive/
  const fecha = new Date().toISOString().split('T')[0];
  const archiveDir = path.join(currentDir, '../data/archive');
  const archivePath = path.join(archiveDir, `obcl_${fecha}.json`);
  
  if (!fs.existsSync(archiveDir)) {
    fs.mkdirSync(archiveDir, { recursive: true });
  }
  
  fs.writeFileSync(archivePath, dataStr);
  console.log(`✅ Histórico guardado en: ${archivePath}`);

  // 2. Sobrescribir el estado actual en /data/current.json
  const currentPath = path.join(currentDir, '../data/current.json');
  fs.writeFileSync(currentPath, dataStr);
  console.log(`✅ Estado actual sobrescrito en: ${currentPath}`);
}

run();