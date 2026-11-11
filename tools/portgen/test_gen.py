"""Tests the file splitting of gen.py. Run with `make test`."""
import unittest

import gen


class SplitPartsTest(unittest.TestCase):
    def test_keeps_one_file_when_the_tests_fit(self):
        self.assertEqual(gen.split_parts("head\n", ["a\n", "b\n"], max_lines=3), ["head\na\nb\n"])

    def test_starts_a_new_file_between_tests(self):
        parts = gen.split_parts("head\n", ["a\na\n", "b\nb\n"], max_lines=3)
        self.assertEqual(parts, ["head\na\na\n", "head\nb\nb\n"])

    def test_keeps_a_test_longer_than_the_limit_whole(self):
        self.assertEqual(gen.split_parts("head\n", ["a\na\na\n"], max_lines=2), ["head\na\na\na\n"])


class PartPathTest(unittest.TestCase):
    def test_names_the_parts_after_the_first(self):
        self.assertEqual(gen.part_path("cmds", "app", 0), "cmds/app_test.go")
        self.assertEqual(gen.part_path("cmds", "app", 1), "cmds/app_part2_test.go")


if __name__ == "__main__":
    unittest.main()
