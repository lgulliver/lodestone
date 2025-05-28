using System;
using Lodestone.TestLibrary;

namespace TestConsole
{
    class Program
    {
        static void Main(string[] args)
        {
            Console.WriteLine("=== Lodestone Test Library Demo ===");
            Console.WriteLine();

            // Test Calculator
            var calc = new Calculator();
            Console.WriteLine("Testing Calculator:");
            Console.WriteLine($"5 + 3 = {calc.Add(5, 3)}");
            Console.WriteLine($"10 - 4 = {calc.Subtract(10, 4)}");
            Console.WriteLine($"6 * 7 = {calc.Multiply(6, 7)}");
            Console.WriteLine($"15 / 3 = {calc.Divide(15, 3)}");
            
            Console.WriteLine();

            // Test StringUtils
            Console.WriteLine("Testing StringUtils:");
            Console.WriteLine($"Reversed 'hello' = '{StringUtils.Reverse("hello")}'");
            Console.WriteLine($"Title case 'hello world' = '{StringUtils.ToTitleCase("hello world")}'");
            Console.WriteLine($"Word count 'The quick brown fox' = {StringUtils.WordCount("The quick brown fox")}");

            Console.WriteLine();
            Console.WriteLine("Demo completed successfully!");
        }
    }
}
