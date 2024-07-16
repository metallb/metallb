// Converts a multilang content folder given in the form of "Translation by Filename"
// into the form of "Translation by content directory"
// see https://gohugo.io/content-management/multilingual/#translate-your-content

const fs = require('fs');
const path = require('path');

function getLanguages(directoryPath) {
  const langs = {};
  try {
    const files = fs.readdirSync(directoryPath);
    files.forEach((file) => {
      const filePath = path.join(directoryPath, file);
      const regex = /^(.*?)\.([^.]+)\.([^.]+)$/;
      const match = file.match(regex);
      if (match) {
        const fileName = match[1];
        const fileLang = match[2];
        const fileExtension = match[3];
        langs[fileLang] = true;
      } else {
        try {
          const stats = fs.statSync(filePath);
          if (stats.isDirectory()) {
            Object.assign(langs, getLanguages(filePath));
          }
        } catch (err) {
          console.error(`Error reading file/directory ${filePath}: ${err}`);
        }
      }
    });
  } catch (err) {
    console.error(`Error reading directory ${directoryPath}: ${err}`);
  }
  return langs;
}

function processDirectory(directoryPath, oldDirectory, newDirectory, langs) {
  try {
    const files = fs.readdirSync(directoryPath);
    files.forEach((file) => {
      const relSubDirectory = directoryPath.replace(oldDirectory, '');
      const filePath = path.join(directoryPath, file);
      try {
        const stats = fs.statSync(filePath);
        const langRegex = /^(.*?)\.([^.]+)\.([^.]+)$/;
        const langMatch = file.match(langRegex);
        if (langMatch) {
          if (stats.isDirectory() || stats.isFile()) {
            // language files are be copied over as is
            // language directories are completly copied over as is, no need to scan its content as it already identified for a language
            const fileName = langMatch[1];
            const fileLang = langMatch[2];
            const fileExtension = langMatch[3];
            const newFileDirectory = path.join(newDirectory, fileLang, relSubDirectory);
            const newFilePath = path.join(newFileDirectory, fileName + '.' + fileExtension);
            try {
              fs.mkdirSync(newFileDirectory, { recursive: true });
              fs.cpSync(filePath, newFilePath, { recursive: true });
            } catch (err) {
              console.error(`Error copying file/directory ${filePath} to ${newFilePath}: ${err}`);
              return false;
            }
          }
        } else {
          if (stats.isDirectory()) {
            // non-language directories by itself are irrelevant, but let's check for their content
            if (!processDirectory(filePath, oldDirectory, newDirectory, langs)) {
              return false;
            }
          } else if (stats.isFile()){
            // non-language files are a different beast: copy the file into all languages that don't have a language file
            const nonLangRegex = /^(.*?)(\.([^.]+))?$/;
            const nonLangMatch = file.match(nonLangRegex);
            const fileName = nonLangMatch[1];
            const fileExtension = nonLangMatch.length > 3 ? nonLangMatch[3] : "";
            Object.keys(langs).forEach((fileLang) => {
              const langFilePath = path.join(directoryPath, fileName + '.' + fileLang + '.' + fileExtension);
              const langExists = fs.existsSync(langFilePath);
              if (!langExists) {
                const newFileDirectory = path.join(newDirectory, fileLang, relSubDirectory);
                const newFilePath = path.join(newFileDirectory, fileName + '.' + fileExtension);
                try {
                  fs.mkdirSync(newFileDirectory, { recursive: true });
                  fs.cpSync(filePath, newFilePath, { recursive: true });
                } catch (err) {
                  console.error(`Error copying file ${filePath} to ${newFilePath}: ${err}`);
                  return false;
                }
              }
            });
          }
        }
      } catch (err) {
        console.error(`Error reading file/directory ${filePath}: ${err}`);
        return false;
      }
    });
  } catch (err) {
    console.error(`Error reading directory ${directoryPath}: ${err}`);
    return false;
  }
  return true;
}

function modifyConfig(configDirectory) {

}

function runThatShit(contentDirectory) {
  const sourceDirectory = contentDirectory;
  const backupDirectory = contentDirectory + ".backup";
  const targetDirectory = contentDirectory + ".temp";

  // check directories
  try {
    fs.accessSync(sourceDirectory, fs.constants.W_OK | fs.constants.R_OK);
  } catch (err) {
    console.error(`No read/write permisson for directory ${sourceDirectory}: ${err}`);
    return false;
  }
  /*
  try {
    fs.statSync(backupDirectory);
    console.error(`Backup directory from last failed conversion still present ${backupDirectory}`);
    return false;
  } catch (err) {}
  */

  // Make space for the conversion
  try {
    fs.rmSync(targetDirectory, { recursive: true });
  } catch (err) {
    try {
      fs.statSync(targetDirectory);
      console.error(`Error removing directory ${targetDirectory}: ${err}`);
      return false;
    } catch (err) {}
  }
  try {
    fs.rmSync(backupDirectory, { recursive: true });
  } catch (err) {
    try {
      fs.statSync(backupDirectory);
      console.error(`Error removing directory ${backupDirectory}: ${err}`);
      return false;
    } catch (err) {}
  }

  // convert that shit
  const langs = getLanguages(sourceDirectory);
  if( !processDirectory(sourceDirectory, sourceDirectory, targetDirectory, langs) ){
    return false;
  }

  // move final result around
  try {
    fs.renameSync(sourceDirectory, backupDirectory);
    fs.renameSync(targetDirectory, sourceDirectory);
    fs.rmSync(backupDirectory, { recursive: true });
  } catch (err) {
    console.error(`Error deleting/renaming directories: ${err}`);
    return false;
  }

  // edit the config file accordingly
  modifyConfig(path.join(sourceDirectory, ".."));
  return true;
}

const contentDirectory = path.join(__dirname, 'content');
runThatShit(contentDirectory);
