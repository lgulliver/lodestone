using System;

namespace Lodestone.TestLibrary
{
    /// <summary>
    /// A simple calculator class for testing NuGet package functionality
    /// </summary>
    public class Calculator
    {
        /// <summary>
        /// Adds two integers
        /// </summary>
        /// <param name="a">First number</param>
        /// <param name="b">Second number</param>
        /// <returns>Sum of a and b</returns>
        public int Add(int a, int b)
        {
            Console.WriteLine($"Adding {a} + {b}");
            return a + b;
        }

        /// <summary>
        /// Subtracts two integers
        /// </summary>
        /// <param name="a">First number</param>
        /// <param name="b">Second number</param>
        /// <returns>Difference of a and b</returns>
        public int Subtract(int a, int b)
        {
            Console.WriteLine($"Subtracting {a} - {b}");
            return a - b;
        }

        /// <summary>
        /// Multiplies two integers
        /// </summary>
        /// <param name="a">First number</param>
        /// <param name="b">Second number</param>
        /// <returns>Product of a and b</returns>
        public int Multiply(int a, int b)
        {
            Console.WriteLine($"Multiplying {a} * {b}");
            return a * b;
        }

        /// <summary>
        /// Divides two integers
        /// </summary>
        /// <param name="a">Dividend</param>
        /// <param name="b">Divisor</param>
        /// <returns>Quotient of a and b</returns>
        /// <exception cref="DivideByZeroException">Thrown when b is zero</exception>
        public double Divide(int a, int b)
        {
            if (b == 0)
                throw new DivideByZeroException("Cannot divide by zero");
                
            Console.WriteLine($"Dividing {a} / {b}");
            return (double)a / b;
        }
    }
}
