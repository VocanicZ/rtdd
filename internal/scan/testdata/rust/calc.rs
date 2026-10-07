pub enum Op {
    Add,
    Sub,
}

pub trait Apply {
    fn apply(&self, a: i32, b: i32) -> i32;
}

pub struct Calc {
    total: i32,
}

impl Calc {
    pub fn new() -> Self {
        Calc { total: 0 }
    }

    pub fn push(&mut self, n: i32) {
        self.total = add(self.total, n);
    }
}

impl Apply for Op {
    fn apply(&self, a: i32, b: i32) -> i32 {
        match self {
            Op::Add => add(a, b),
            Op::Sub => a - b,
        }
    }
}

pub fn add(a: i32, b: i32) -> i32 {
    a + b
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn adds() {
        assert_eq!(add(1, 2), 3);
    }

    #[test]
    fn pushes() {
        let mut c = Calc::new();
        c.push(2);
        assert_eq!(c.total, 2);
    }

    #[test]
    fn applies() {
        assert_eq!(Op::Sub.apply(3, 1), 2);
    }
}

fn greet(name: &'static str) -> String {
    format!("hi {}", name)
}

fn other() -> u32 {
    1
}

fn third() -> u32 {
    other()
}
