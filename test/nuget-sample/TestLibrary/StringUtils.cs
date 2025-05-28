using System;

namespace Lodestone.TestLibrary
{
    /// <summary>
    /// Utility functions for string operations
    /// </summary>
    public static class StringUtils
    {
        /// <summary>
        /// Reverses a string
        /// </summary>
        /// <param name="input">Input string</param>
        /// <returns>Reversed string</returns>
        public static string Reverse(string input)
        {
            if (string.IsNullOrEmpty(input))
                return input;

            char[] chars = input.ToCharArray();
            Array.Reverse(chars);
            Console.WriteLine($"Reversed '{input}' to '{new string(chars)}'");
            return new string(chars);
        }

        /// <summary>
        /// Converts string to title case
        /// </summary>
        /// <param name="input">Input string</param>
        /// <returns>Title cased string</returns>
        public static string ToTitleCase(string input)
        {
            if (string.IsNullOrEmpty(input))
                return input;

            var result = System.Globalization.CultureInfo.CurrentCulture.TextInfo.ToTitleCase(input.ToLower());
            Console.WriteLine($"Converted '{input}' to title case: '{result}'");
            return result;
        }

        /// <summary>
        /// Counts the number of words in a string
        /// </summary>
        /// <param name="input">Input string</param>
        /// <returns>Number of words</returns>
        public static int WordCount(string input)
        {
            if (string.IsNullOrWhiteSpace(input))
                return 0;

            var words = input.Split(new char[] { ' ', '\t', '\n', '\r' }, 
                StringSplitOptions.RemoveEmptyEntries);
            
            Console.WriteLine($"Found {words.Length} words in '{input}'");
            return words.Length;
        }
    }
}
