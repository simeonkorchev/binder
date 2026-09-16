// Monorepo resolution: Metro must watch the workspace root (npm hoists most
// dependencies there) and look for modules in both node_modules trees.
// 005-mobile.md: do not remove watchFolders / nodeModulesPaths.
const path = require('node:path')

const { getDefaultConfig } = require('expo/metro-config')

const projectRoot = __dirname
const workspaceRoot = path.resolve(projectRoot, '../..')

const config = getDefaultConfig(projectRoot)

config.watchFolders = [workspaceRoot]
config.resolver.nodeModulesPaths = [
  path.resolve(projectRoot, 'node_modules'),
  path.resolve(workspaceRoot, 'node_modules'),
]

module.exports = config
