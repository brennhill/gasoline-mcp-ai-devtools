import assert from 'node:assert/strict'
import test from 'node:test'

import {
  normalizeMainPyprojectContent,
  validateMainPyprojectContent
} from './normalize-pypi-main-pyproject.js'

const version = '0.7.2'
const baseProject = `[project]
name = "gasoline-mcp"
version = "${version}"
description = "Main package"

[project.urls]
Homepage = "https://cookwithgasoline.com"

[project.scripts]
gasoline-mcp = "gasoline_mcp.__main__:main"
`

test('normalizer moves dependencies from [project.scripts] to [project]', () => {
  const input = `${baseProject}
dependencies = [
    "gasoline-mcp-darwin-arm64==${version}; sys_platform == 'darwin' and platform_machine == 'arm64'",
    "gasoline-mcp-darwin-x64==${version}; sys_platform == 'darwin' and platform_machine == 'x86_64'",
    "gasoline-mcp-linux-arm64==${version}; sys_platform == 'linux' and platform_machine == 'aarch64'",
    "gasoline-mcp-linux-x64==${version}; sys_platform == 'linux' and platform_machine == 'x86_64'",
    "gasoline-mcp-win32-x64==${version}; sys_platform == 'win32'",
]
`

  const result = normalizeMainPyprojectContent(input)
  assert.equal(result.changed, true)
  assert.match(result.content, /\[project\][\s\S]*dependencies = \[/)
  assert.match(
    result.content,
    /\[project\.scripts\]\ngasoline-mcp = "gasoline_mcp\.__main__:main"\n?$/
  )
  assert.equal(
    result.content.match(/dependencies = \[/g)?.length ?? 0,
    1,
    'dependencies block should exist exactly once'
  )
})

test('normalizer removes duplicate dependencies under [project.scripts] when [project] already has one', () => {
  const input = `[project]
name = "gasoline-mcp"
version = "${version}"
dependencies = [
    "gasoline-mcp-win32-x64==${version}; sys_platform == 'win32'",
]

[project.scripts]
gasoline-mcp = "gasoline_mcp.__main__:main"
dependencies = [
    "gasoline-mcp-win32-x64==${version}; sys_platform == 'win32'",
]
`
  const result = normalizeMainPyprojectContent(input)
  assert.equal(result.changed, true)
  assert.equal(result.content.match(/dependencies = \[/g)?.length ?? 0, 1)
})

test('normalizer leaves already-correct structure unchanged', () => {
  const input = `[project]
name = "gasoline-mcp"
version = "${version}"
dependencies = [
    "gasoline-mcp-win32-x64==${version}; sys_platform == 'win32'",
]

[project.scripts]
gasoline-mcp = "gasoline_mcp.__main__:main"
`

  const result = normalizeMainPyprojectContent(input)
  assert.equal(result.changed, false)
  assert.equal(result.content, input)
})

test('validator rejects misplaced dependencies and accepts normalized content', () => {
  const misplaced = `${baseProject}
dependencies = [
    "gasoline-mcp-win32-x64==${version}; sys_platform == 'win32'",
]
`
  const invalid = validateMainPyprojectContent(misplaced, { expectedVersion: version })
  assert.equal(invalid.valid, false)
  assert.match(invalid.errors.join('\n'), /\[project\.scripts\] must not define dependencies/)

  const normalized = `[project]
name = "gasoline-mcp"
version = "${version}"
dependencies = [
    "gasoline-mcp-darwin-arm64==${version}; sys_platform == 'darwin' and platform_machine == 'arm64'",
    "gasoline-mcp-darwin-x64==${version}; sys_platform == 'darwin' and platform_machine == 'x86_64'",
    "gasoline-mcp-linux-arm64==${version}; sys_platform == 'linux' and platform_machine == 'aarch64'",
    "gasoline-mcp-linux-x64==${version}; sys_platform == 'linux' and platform_machine == 'x86_64'",
    "gasoline-mcp-win32-x64==${version}; sys_platform == 'win32'",
]

[project.scripts]
gasoline-mcp = "gasoline_mcp.__main__:main"
`
  const valid = validateMainPyprojectContent(normalized, { expectedVersion: version })
  assert.equal(valid.valid, true, valid.errors.join('\n'))
})
