# Extracts each `test "name" do ... end` of a Minitest file as JSON: statements with
# their source, literal values evaluated by Ruby, and expected strings.
require "prism"
require "json"

def literal(node)
  return nil unless node
  src = node.slice
  return { "value" => eval(src) } if node.is_a?(Prism::StringNode) || node.is_a?(Prism::HashNode) || node.is_a?(Prism::ArrayNode) ||
    node.is_a?(Prism::IntegerNode) || node.is_a?(Prism::TrueNode) || node.is_a?(Prism::FalseNode) || node.is_a?(Prism::NilNode) || node.is_a?(Prism::SymbolNode)
  if node.is_a?(Prism::InterpolatedStringNode)
    return { "interpolated" => src }
  end
  nil
rescue StandardError, SyntaxError
  nil
end

def statement(node)
  out = { "src" => node.slice, "line" => node.location.start_line }
  if node.is_a?(Prism::CallNode)
    out["call"] = node.name.to_s
    args = node.arguments&.arguments || []
    out["args"] = args.map { |a| { "src" => a.slice }.merge(literal(a) || {}) }
    out["receiver"] = node.receiver&.slice
    out["block"] = node.block&.slice if node.block
  end
  out
end

file = ARGV[0]
tree = Prism.parse(File.read(file)).value
tests = []
setup = nil
visit = lambda do |node|
  return unless node.is_a?(Prism::Node)
  if node.is_a?(Prism::CallNode) && node.name == :test && node.block
    name = node.arguments.arguments.first
    body = node.block.body
    stmts = body ? (body.is_a?(Prism::StatementsNode) ? body.body : (body.respond_to?(:statements) && body.statements ? body.statements.body : [body])) : []
    tests << { "name" => (name.is_a?(Prism::StringNode) ? name.unescaped : name.slice), "line" => node.location.start_line, "statements" => stmts.map { |s| statement(s) } }
    return
  end
  if node.is_a?(Prism::CallNode) && node.name == :setup && node.block && setup.nil?
    setup = node.block.slice
  end
  node.compact_child_nodes.each { |c| visit.call(c) }
end
visit.call(tree)
puts JSON.pretty_generate({ "file" => file, "setup" => setup, "tests" => tests })
