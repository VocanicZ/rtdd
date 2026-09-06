#!/usr/bin/env bash
# Regenerates internal/report/testdata/junit/*.xml by RUNNING nine test runners.
#
# The fixtures under that directory are the ground truth internal/report's
# junit-xml parser is tested against, and they are worth only as much as their
# provenance: a hand-written fixture encodes what its author believes JUnit XML
# looks like, which is the belief the format keeps falsifying. So every fixture
# is captured output — this script builds a three-test suite (one pass, one
# fail, one skip) for each ecosystem, runs the runner over it in a pinned
# container, and copies the runner's own file out unedited.
#
# Not part of the CI gate: it needs Docker and the network, and the fixtures it
# writes are committed. Run it when a runner's output shape needs re-checking
# against a newer version, then read the diff before committing it.
#
# Usage: scripts/capture-junit-fixtures.sh [runner ...]   (default: all nine)
set -euo pipefail

cd "$(dirname "$0")/.."
DEST="$PWD/internal/report/testdata/junit"
mkdir -p "$DEST"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker not found on PATH — this script runs each runner in a pinned image" >&2
  exit 1
fi

# Every image is pinned by tag so a re-capture changes the fixture only when the
# author intends it to. The versions the committed fixtures were captured with
# are recorded in internal/report/testdata/junit/README.md.
NODE_IMAGE="node:22-slim"
GO_IMAGE="golang:1.24"
MAVEN_IMAGE="maven:3.9-eclipse-temurin-21"
RUBY_IMAGE="ruby:3.3-slim"
PHP_IMAGE="php:8.3-cli"
RUST_IMAGE="rust:1-slim"
DOTNET_IMAGE="mcr.microsoft.com/dotnet/sdk:8.0"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# run_in mounts one prepared project at /work and runs the capture command as the
# invoking user, so the files that come back are not owned by root.
run_in() {
  local image="$1" dir="$2" script="$3"
  docker run --rm \
    -u "$(id -u):$(id -g)" \
    -e HOME=/tmp \
    -v "$dir:/work" \
    -w /work \
    "$image" bash -c "$script"
}

capture_vitest() {
  local d="$WORK/vitest"
  mkdir -p "$d/test"
  cat >"$d/package.json" <<'EOF'
{ "name": "rtdd-junit-fixture", "private": true, "type": "module" }
EOF
  cat >"$d/src.js" <<'EOF'
export function add(a, b) { return a + b; }
EOF
  cat >"$d/test/calc.test.js" <<'EOF'
import { describe, it, expect } from 'vitest';
import { add } from '../src.js';

describe('calculator', () => {
  it('adds two numbers', () => {
    expect(add(1, 2)).toBe(3);
  });

  it('fails on a wrong sum', () => {
    expect(add(1, 2)).toBe(4);
  });

  it.skip('is not implemented yet', () => {
    expect(add(1, 1)).toBe(2);
  });
});
EOF
  run_in "$NODE_IMAGE" "$d" '
    set -e
    npm install --no-audit --no-fund -D vitest@3.2.7 >/dev/null
    npx vitest run --reporter=junit --outputFile=./junit.xml || true
    npx vitest --version'
  cp "$d/junit.xml" "$DEST/vitest.xml"
}

# jest_project writes the three-test suite both jest captures run, into the
# directory named. The two differ ONLY in jest-junit's classNameTemplate, which is
# what jest-filepath.xml exists to show: same runner, same suite, one env var.
jest_project() {
  local d="$1"
  mkdir -p "$d/test"
  cat >"$d/package.json" <<'EOF'
{ "name": "rtdd-junit-fixture", "private": true }
EOF
  cat >"$d/src.js" <<'EOF'
function add(a, b) { return a + b; }
module.exports = { add };
EOF
  cat >"$d/test/calc.test.js" <<'EOF'
const { add } = require('../src.js');

describe('calculator', () => {
  test('adds two numbers', () => {
    expect(add(1, 2)).toBe(3);
  });

  test('fails on a wrong sum', () => {
    expect(add(1, 2)).toBe(4);
  });

  test.skip('is not implemented yet', () => {
    expect(add(1, 1)).toBe(2);
  });
});
EOF
}

capture_jest() {
  local d="$WORK/jest"
  jest_project "$d"
  run_in "$NODE_IMAGE" "$d" '
    set -e
    npm install --no-audit --no-fund -D jest@29.7.0 jest-junit@16.0.0 >/dev/null
    npx jest --reporters=jest-junit || true
    npx jest --version'
  cp "$d/junit.xml" "$DEST/jest.xml"
}

# jest-junit's default classNameTemplate is "{classname} {title}", which is the
# test's own full name — so jest.xml's classname and name are the same string and
# no template over it names the FILE. adapters/jest.yaml therefore ships
# `report_cmd`-free but reconfigured: JEST_JUNIT_CLASSNAME='{filepath}' makes
# classname the test file's path, which is the id_template the adapter declares.
# This capture is that configuration's evidence.
capture_jest_filepath() {
  local d="$WORK/jest-filepath"
  jest_project "$d"
  run_in "$NODE_IMAGE" "$d" '
    set -e
    npm install --no-audit --no-fund -D jest@29.7.0 jest-junit@16.0.0 >/dev/null
    JEST_JUNIT_CLASSNAME="{filepath}" npx jest --reporters=jest-junit || true
    npx jest --version'
  cp "$d/junit.xml" "$DEST/jest-filepath.xml"
}

capture_go() {
  local d="$WORK/go"
  mkdir -p "$d/calc"
  cat >"$d/go.mod" <<'EOF'
module example.com/calc

go 1.24
EOF
  cat >"$d/calc/calc.go" <<'EOF'
package calc

func Add(a, b int) int { return a + b }
EOF
  cat >"$d/calc/calc_test.go" <<'EOF'
package calc

import (
	"testing"
	"time"
)

func TestAddsTwoNumbers(t *testing.T) {
	// `go test -json` reports elapsed seconds to two decimals, so a test that
	// finishes instantly is time="0.000" in the report. The sleep is what makes
	// this fixture carry a non-zero duration to assert on.
	time.Sleep(20 * time.Millisecond)
	if got := Add(1, 2); got != 3 {
		t.Fatalf("Add(1, 2) = %d, want 3", got)
	}
}

func TestFailsOnAWrongSum(t *testing.T) {
	if got := Add(1, 2); got != 4 {
		t.Fatalf("Add(1, 2) = %d, want 4", got)
	}
}

func TestIsNotImplementedYet(t *testing.T) {
	t.Skip("not implemented yet")
}
EOF
  run_in "$GO_IMAGE" "$d" '
    set -e
    export GOFLAGS=-mod=mod GOPATH=/tmp/go GOCACHE=/tmp/gocache
    go install github.com/jstemmer/go-junit-report/v2@v2.1.0 >/dev/null 2>&1
    go test -json ./... > gotest.json || true
    /tmp/go/bin/go-junit-report -parser gojson < gotest.json > junit.xml
    /tmp/go/bin/go-junit-report -version'
  cp "$d/junit.xml" "$DEST/go-junit-report.xml"
}

capture_surefire() {
  local d="$WORK/surefire"
  mkdir -p "$d/src/main/java/calc" "$d/src/test/java/calc"
  cat >"$d/pom.xml" <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>calc</artifactId>
  <version>1.0</version>
  <properties>
    <maven.compiler.release>21</maven.compiler.release>
    <project.build.sourceEncoding>UTF-8</project.build.sourceEncoding>
  </properties>
  <dependencies>
    <dependency>
      <groupId>org.junit.jupiter</groupId>
      <artifactId>junit-jupiter</artifactId>
      <version>5.10.2</version>
      <scope>test</scope>
    </dependency>
  </dependencies>
  <build>
    <plugins>
      <plugin>
        <groupId>org.apache.maven.plugins</groupId>
        <artifactId>maven-surefire-plugin</artifactId>
        <version>3.2.5</version>
      </plugin>
    </plugins>
  </build>
</project>
EOF
  cat >"$d/src/main/java/calc/Calc.java" <<'EOF'
package calc;

public class Calc {
    public static int add(int a, int b) {
        return a + b;
    }
}
EOF
  cat >"$d/src/test/java/calc/CalcTest.java" <<'EOF'
package calc;

import static org.junit.jupiter.api.Assertions.assertEquals;

import org.junit.jupiter.api.Disabled;
import org.junit.jupiter.api.Test;

class CalcTest {
    @Test
    void addsTwoNumbers() {
        assertEquals(3, Calc.add(1, 2));
    }

    @Test
    void failsOnAWrongSum() {
        assertEquals(4, Calc.add(1, 2));
    }

    @Test
    @Disabled("not implemented yet")
    void isNotImplementedYet() {
        assertEquals(2, Calc.add(1, 1));
    }
}
EOF
  run_in "$MAVEN_IMAGE" "$d" '
    set -e
    mvn -B -q -Dmaven.repo.local=/tmp/m2 test || true
    mvn -B -version | head -1'
  cp "$d/target/surefire-reports/TEST-calc.CalcTest.xml" "$DEST/surefire.xml"
}

capture_rspec() {
  local d="$WORK/rspec"
  mkdir -p "$d/lib" "$d/spec"
  cat >"$d/lib/calc.rb" <<'EOF'
module Calc
  def self.add(a, b)
    a + b
  end
end
EOF
  cat >"$d/spec/calc_spec.rb" <<'EOF'
require_relative '../lib/calc'

RSpec.describe Calc do
  it 'adds two numbers' do
    expect(Calc.add(1, 2)).to eq(3)
  end

  it 'fails on a wrong sum' do
    expect(Calc.add(1, 2)).to eq(4)
  end

  it 'is not implemented yet' do
    skip 'not implemented yet'
    expect(Calc.add(1, 1)).to eq(2)
  end
end
EOF
  run_in "$RUBY_IMAGE" "$d" '
    set -e
    export GEM_HOME=/tmp/gems PATH=/tmp/gems/bin:$PATH
    gem install --no-document rspec:3.13.0 rspec_junit_formatter:0.6.0 >/dev/null
    rspec --format RspecJunitFormatter --out junit.xml || true
    rspec --version | head -1'
  cp "$d/junit.xml" "$DEST/rspec.xml"
}

capture_phpunit() {
  local d="$WORK/phpunit"
  mkdir -p "$d/src" "$d/tests"
  cat >"$d/src/Calc.php" <<'EOF'
<?php

class Calc
{
    public static function add(int $a, int $b): int
    {
        return $a + $b;
    }
}
EOF
  cat >"$d/tests/CalcTest.php" <<'EOF'
<?php

require_once __DIR__ . '/../src/Calc.php';

use PHPUnit\Framework\TestCase;

class CalcTest extends TestCase
{
    public function testAddsTwoNumbers(): void
    {
        $this->assertSame(3, Calc::add(1, 2));
    }

    public function testFailsOnAWrongSum(): void
    {
        $this->assertSame(4, Calc::add(1, 2));
    }

    public function testIsNotImplementedYet(): void
    {
        $this->markTestSkipped('not implemented yet');
    }
}
EOF
  run_in "$PHP_IMAGE" "$d" '
    set -e
    php -r "copy(\"https://phar.phpunit.de/phpunit-10.5.20.phar\", \"phpunit.phar\");"
    php phpunit.phar --log-junit junit.xml tests || true
    php phpunit.phar --version | head -1'
  cp "$d/junit.xml" "$DEST/phpunit.xml"
  rm -f "$d/phpunit.phar"
}

# cargo-nextest is the only shipped runner whose JUnit output is written by the
# test HARNESS rather than by a reporter plugin: `[profile.ci.junit] path` in
# .config/nextest.toml turns it on, and nextest writes it under
# target/nextest/<profile>/. `cargo install` is what builds it here because
# rust:1-slim carries neither curl nor wget and the container is not root, so the
# prebuilt tarball nexte.st publishes cannot be fetched — the version is pinned
# either way, which is what the fixture's provenance needs.
capture_nextest() {
  local d="$WORK/nextest"
  mkdir -p "$d/src" "$d/.config"
  cat >"$d/Cargo.toml" <<'EOF'
[package]
name = "calc"
version = "0.1.0"
edition = "2021"
EOF
  cat >"$d/.config/nextest.toml" <<'EOF'
[profile.ci.junit]
path = "junit.xml"
EOF
  cat >"$d/src/lib.rs" <<'EOF'
pub fn add(a: i32, b: i32) -> i32 {
    a + b
}

#[cfg(test)]
mod tests {
    use super::add;

    #[test]
    fn adds_two_numbers() {
        assert_eq!(add(1, 2), 3);
    }

    #[test]
    fn fails_on_a_wrong_sum() {
        assert_eq!(add(1, 2), 4);
    }

    #[test]
    #[ignore = "not implemented yet"]
    fn is_not_implemented_yet() {
        assert_eq!(add(1, 1), 2);
    }
}
EOF
  run_in "$RUST_IMAGE" "$d" '
    set -e
    export CARGO_HOME=/tmp/cargo PATH=/tmp/cargo/bin:$PATH CARGO_TARGET_DIR=/work/target
    cargo install --quiet --locked cargo-nextest@0.9.78
    cargo nextest run --profile ci || true
    cargo nextest --version'
  cp "$d/target/nextest/ci/junit.xml" "$DEST/nextest.xml"
}

# .NET has no built-in JUnit writer — `dotnet test` ships TRX — so the adapter
# declares the JunitXml.TestLogger package and `--logger junit`, and this capture
# is that combination's output. VSTest writes the logger file under the results
# directory, which is why adapters/dotnet.yaml declares a DIRECTORY report_path.
capture_dotnet() {
  local d="$WORK/dotnet"
  mkdir -p "$d"
  cat >"$d/Calc.Tests.csproj" <<'EOF'
<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
    <Nullable>enable</Nullable>
    <IsPackable>false</IsPackable>
    <RootNamespace>Calc.Tests</RootNamespace>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="Microsoft.NET.Test.Sdk" Version="17.9.0" />
    <PackageReference Include="xunit" Version="2.7.0" />
    <PackageReference Include="xunit.runner.visualstudio" Version="2.5.7" />
    <PackageReference Include="JunitXml.TestLogger" Version="3.1.12" />
  </ItemGroup>
</Project>
EOF
  cat >"$d/Calc.cs" <<'EOF'
namespace Calc.Tests;

public static class Calc
{
    public static int Add(int a, int b) => a + b;
}
EOF
  cat >"$d/CalcTest.cs" <<'EOF'
using Xunit;

namespace Calc.Tests;

public class CalcTest
{
    [Fact]
    public void AddsTwoNumbers()
    {
        Assert.Equal(3, Calc.Add(1, 2));
    }

    [Fact]
    public void FailsOnAWrongSum()
    {
        Assert.Equal(4, Calc.Add(1, 2));
    }

    [Fact(Skip = "not implemented yet")]
    public void IsNotImplementedYet()
    {
        Assert.Equal(2, Calc.Add(1, 1));
    }
}
EOF
  run_in "$DOTNET_IMAGE" "$d" '
    set -e
    export DOTNET_CLI_HOME=/tmp DOTNET_CLI_TELEMETRY_OPTOUT=1 DOTNET_NOLOGO=1
    export NUGET_PACKAGES=/tmp/nuget
    dotnet test --logger junit --results-directory /work/TestResults || true
    dotnet --version'
  cp "$d/TestResults/TestResults.xml" "$DEST/dotnet.xml"
}

runners=("$@")
if [ ${#runners[@]} -eq 0 ]; then
  runners=(vitest jest jest-filepath go surefire rspec phpunit nextest dotnet)
fi

for r in "${runners[@]}"; do
  echo "==> capturing $r"
  case "$r" in
    vitest) capture_vitest ;;
    jest) capture_jest ;;
    jest-filepath) capture_jest_filepath ;;
    go) capture_go ;;
    surefire) capture_surefire ;;
    rspec) capture_rspec ;;
    phpunit) capture_phpunit ;;
    nextest) capture_nextest ;;
    dotnet) capture_dotnet ;;
    *) echo "unknown runner: $r" >&2; exit 1 ;;
  esac
done

echo "==> fixtures written to $DEST"
