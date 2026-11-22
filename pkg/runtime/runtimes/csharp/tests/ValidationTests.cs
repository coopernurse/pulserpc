using System;
using System.Collections.Generic;
using Xunit;
using PulseRPC;

namespace PulseRPC.Tests
{
    public class BuiltInTypesTests
    {
        [Fact]
        public void ValidateString_Success()
        {
            Validation.ValidateString("hello");
            Validation.ValidateString("");
        }

        [Fact]
        public void ValidateString_Failure()
        {
            Assert.Throws<ArgumentException>(() => Validation.ValidateString(123));
            Assert.Throws<ArgumentException>(() => Validation.ValidateString(null));
        }

        [Fact]
        public void ValidateInt_Success()
        {
            Validation.ValidateInt(0);
            Validation.ValidateInt(42);
            Validation.ValidateInt(-100);
        }

        [Fact]
        public void ValidateInt_Failure()
        {
            Assert.Throws<ArgumentException>(() => Validation.ValidateInt("123"));
            Assert.Throws<ArgumentException>(() => Validation.ValidateInt(3.14));
        }

        [Fact]
        public void ValidateFloat_Success()
        {
            Validation.ValidateFloat(3.14);
            Validation.ValidateFloat(42); // int is acceptable
            Validation.ValidateFloat(-1.5);
        }

        [Fact]
        public void ValidateFloat_Failure()
        {
            Assert.Throws<ArgumentException>(() => Validation.ValidateFloat("3.14"));
            Assert.Throws<ArgumentException>(() => Validation.ValidateFloat(null));
        }

        [Fact]
        public void ValidateBool_Success()
        {
            Validation.ValidateBool(true);
            Validation.ValidateBool(false);
        }

        [Fact]
        public void ValidateBool_Failure()
        {
            Assert.Throws<ArgumentException>(() => Validation.ValidateBool(1));
            Assert.Throws<ArgumentException>(() => Validation.ValidateBool("true"));
        }
    }

    public class ArrayValidationTests
    {
        [Fact]
        public void ValidateArray_Success()
        {
            Validation.ValidateArray(new[] { "a", "b", "c" }, Validation.ValidateString);
            Validation.ValidateArray(new string[] { }, Validation.ValidateString);
        }

        [Fact]
        public void ValidateArray_WrongType()
        {
            Assert.Throws<ArgumentException>(() => Validation.ValidateArray("not a list", Validation.ValidateString));
            Assert.Throws<ArgumentException>(() => Validation.ValidateArray(new Dictionary<string, object>(), Validation.ValidateString));
        }

        [Fact]
        public void ValidateArray_ElementValidationFails()
        {
            Assert.Throws<ArgumentException>(() => 
                Validation.ValidateArray(new object[] { "a", 123, "c" }, Validation.ValidateString));
        }
    }

    public class MapValidationTests
    {
        [Fact]
        public void ValidateMap_Success()
        {
            var map = new Dictionary<string, object?> { { "a", 1 }, { "b", 2 } };
            Validation.ValidateMap(map, Validation.ValidateInt);
            Validation.ValidateMap(new Dictionary<string, object?>(), Validation.ValidateInt);
        }

        [Fact]
        public void ValidateMap_WrongType()
        {
            Assert.Throws<ArgumentException>(() => Validation.ValidateMap("not a dict", Validation.ValidateInt));
            Assert.Throws<ArgumentException>(() => Validation.ValidateMap(new List<object>(), Validation.ValidateInt));
        }

        [Fact]
        public void ValidateMap_ValueValidationFails()
        {
            var map = new Dictionary<string, object?> { { "a", "not an int" } };
            Assert.Throws<ArgumentException>(() => Validation.ValidateMap(map, Validation.ValidateInt));
        }
    }

    public class EnumValidationTests
    {
        [Fact]
        public void ValidateEnum_Success()
        {
            Validation.ValidateEnum("kindle", "Platform", new List<string> { "kindle", "nook" });
            Validation.ValidateEnum("nook", "Platform", new List<string> { "kindle", "nook" });
        }

        [Fact]
        public void ValidateEnum_WrongType()
        {
            Assert.Throws<ArgumentException>(() => 
                Validation.ValidateEnum(123, "Platform", new List<string> { "kindle", "nook" }));
        }

        [Fact]
        public void ValidateEnum_InvalidValue()
        {
            Assert.Throws<ArgumentException>(() => 
                Validation.ValidateEnum("invalid", "Platform", new List<string> { "kindle", "nook" }));
        }
    }

    public class StructValidationTests
    {
        [Fact]
        public void ValidateStruct_Success()
        {
            var allStructs = new Dictionary<string, Dictionary<string, object>>
            {
                {
                    "User",
                    new Dictionary<string, object>
                    {
                        {
                            "fields",
                            new List<Dictionary<string, object>>
                            {
                                new Dictionary<string, object>
                                {
                                    { "name", "id" },
                                    { "type", new Dictionary<string, object> { { "builtIn", "string" } } },
                                    { "optional", false }
                                },
                                new Dictionary<string, object>
                                {
                                    { "name", "name" },
                                    { "type", new Dictionary<string, object> { { "builtIn", "string" } } },
                                    { "optional", false }
                                }
                            }
                        }
                    }
                }
            };
            var allEnums = new Dictionary<string, Dictionary<string, object>>();
            var structDef = allStructs["User"];

            var value = new Dictionary<string, object?> { { "id", "123" }, { "name", "Alice" } };
            Validation.ValidateStruct(value, "User", structDef, allStructs, allEnums);
        }

        [Fact]
        public void ValidateStruct_MissingRequiredField()
        {
            var allStructs = new Dictionary<string, Dictionary<string, object>>
            {
                {
                    "User",
                    new Dictionary<string, object>
                    {
                        {
                            "fields",
                            new List<Dictionary<string, object>>
                            {
                                new Dictionary<string, object>
                                {
                                    { "name", "id" },
                                    { "type", new Dictionary<string, object> { { "builtIn", "string" } } },
                                    { "optional", false }
                                }
                            }
                        }
                    }
                }
            };
            var allEnums = new Dictionary<string, Dictionary<string, object>>();
            var structDef = allStructs["User"];

            Assert.Throws<ArgumentException>(() => 
                Validation.ValidateStruct(new Dictionary<string, object?>(), "User", structDef, allStructs, allEnums));
        }

        [Fact]
        public void ValidateStruct_OptionalField()
        {
            var allStructs = new Dictionary<string, Dictionary<string, object>>
            {
                {
                    "User",
                    new Dictionary<string, object>
                    {
                        {
                            "fields",
                            new List<Dictionary<string, object>>
                            {
                                new Dictionary<string, object>
                                {
                                    { "name", "id" },
                                    { "type", new Dictionary<string, object> { { "builtIn", "string" } } },
                                    { "optional", false }
                                },
                                new Dictionary<string, object>
                                {
                                    { "name", "email" },
                                    { "type", new Dictionary<string, object> { { "builtIn", "string" } } },
                                    { "optional", true }
                                }
                            }
                        }
                    }
                }
            };
            var allEnums = new Dictionary<string, Dictionary<string, object>>();
            var structDef = allStructs["User"];

            // Should work without optional field
            var value1 = new Dictionary<string, object?> { { "id", "123" } };
            Validation.ValidateStruct(value1, "User", structDef, allStructs, allEnums);

            // Should work with optional field
            var value2 = new Dictionary<string, object?> { { "id", "123" }, { "email", "alice@example.com" } };
            Validation.ValidateStruct(value2, "User", structDef, allStructs, allEnums);
        }

        [Fact]
        public void ValidateStruct_WithExtends()
        {
            var allStructs = new Dictionary<string, Dictionary<string, object>>
            {
                {
                    "Base",
                    new Dictionary<string, object>
                    {
                        {
                            "fields",
                            new List<Dictionary<string, object>>
                            {
                                new Dictionary<string, object>
                                {
                                    { "name", "id" },
                                    { "type", new Dictionary<string, object> { { "builtIn", "string" } } },
                                    { "optional", false }
                                }
                            }
                        }
                    }
                },
                {
                    "User",
                    new Dictionary<string, object>
                    {
                        { "extends", "Base" },
                        {
                            "fields",
                            new List<Dictionary<string, object>>
                            {
                                new Dictionary<string, object>
                                {
                                    { "name", "name" },
                                    { "type", new Dictionary<string, object> { { "builtIn", "string" } } },
                                    { "optional", false }
                                }
                            }
                        }
                    }
                }
            };
            var allEnums = new Dictionary<string, Dictionary<string, object>>();
            var structDef = allStructs["User"];

            // Should validate both parent and child fields
            var value = new Dictionary<string, object?> { { "id", "123" }, { "name", "Alice" } };
            Validation.ValidateStruct(value, "User", structDef, allStructs, allEnums);

            // Should fail if parent field missing
            Assert.Throws<ArgumentException>(() => 
                Validation.ValidateStruct(new Dictionary<string, object?> { { "name", "Alice" } }, "User", structDef, allStructs, allEnums));
        }
    }

    public class TypeValidationTests
    {
        [Fact]
        public void ValidateType_String()
        {
            var allStructs = new Dictionary<string, Dictionary<string, object>>();
            var allEnums = new Dictionary<string, Dictionary<string, object>>();
            var typeDef = new Dictionary<string, object> { { "builtIn", "string" } };
            Validation.ValidateType("hello", typeDef, allStructs, allEnums);
        }

        [Fact]
        public void ValidateType_OptionalNone()
        {
            var allStructs = new Dictionary<string, Dictionary<string, object>>();
            var allEnums = new Dictionary<string, Dictionary<string, object>>();
            var typeDef = new Dictionary<string, object> { { "builtIn", "string" } };
            Validation.ValidateType(null, typeDef, allStructs, allEnums, isOptional: true);

            Assert.Throws<ArgumentException>(() => 
                Validation.ValidateType(null, typeDef, allStructs, allEnums, isOptional: false));
        }

        [Fact]
        public void ValidateType_Array()
        {
            var allStructs = new Dictionary<string, Dictionary<string, object>>();
            var allEnums = new Dictionary<string, Dictionary<string, object>>();
            var typeDef = new Dictionary<string, object>
            {
                { "array", new Dictionary<string, object> { { "builtIn", "string" } } }
            };
            Validation.ValidateType(new[] { "a", "b" }, typeDef, allStructs, allEnums);

            Assert.Throws<ArgumentException>(() => 
                Validation.ValidateType(new object[] { "a", 123 }, typeDef, allStructs, allEnums));
        }

        [Fact]
        public void ValidateType_Map()
        {
            var allStructs = new Dictionary<string, Dictionary<string, object>>();
            var allEnums = new Dictionary<string, Dictionary<string, object>>();
            var typeDef = new Dictionary<string, object>
            {
                { "mapValue", new Dictionary<string, object> { { "builtIn", "int" } } }
            };
            var map = new Dictionary<string, object?> { { "a", 1 }, { "b", 2 } };
            Validation.ValidateType(map, typeDef, allStructs, allEnums);

            Assert.Throws<ArgumentException>(() => 
                Validation.ValidateType(new Dictionary<string, object?> { { "a", "not int" } }, typeDef, allStructs, allEnums));
        }

        // csharp-10: enum values are strings on the wire; integers must be rejected.
        [Fact]
        public void ValidateEnum_IntegerValue_IsRejected()
        {
            Assert.Throws<ArgumentException>(() =>
                Validation.ValidateEnum(0, "Color", new List<string> { "red", "green" }));
            Assert.Throws<ArgumentException>(() =>
                Validation.ValidateEnum(1L, "Color", new List<string> { "red", "green" }));
            Validation.ValidateEnum("red", "Color", new List<string> { "red", "green" });
        }

        // csharp-11: document and verify the C# scalar acceptance matrix.
        [Fact]
        public void ScalarAcceptanceMatrix_IntLongDoubleFloatBoolNaNInfinity()
        {
            // int: accepts int, long and whole doubles; rejects bool, NaN and Infinity.
            Validation.ValidateInt(5);
            Validation.ValidateInt(5L);
            Validation.ValidateInt(5.0);
            Assert.Throws<ArgumentException>(() => Validation.ValidateInt(true));
            Assert.Throws<ArgumentException>(() => Validation.ValidateInt(double.NaN));
            Assert.Throws<ArgumentException>(() => Validation.ValidateInt(double.PositiveInfinity));
            Assert.Throws<ArgumentException>(() => Validation.ValidateInt(double.NegativeInfinity));

            // float: accepts int, long, float and double; rejects bool, NaN and Infinity.
            Validation.ValidateFloat(5);
            Validation.ValidateFloat(5L);
            Validation.ValidateFloat(5.0);
            Validation.ValidateFloat(5f);
            Assert.Throws<ArgumentException>(() => Validation.ValidateFloat(true));
            Assert.Throws<ArgumentException>(() => Validation.ValidateFloat(double.NaN));
            Assert.Throws<ArgumentException>(() => Validation.ValidateFloat(double.PositiveInfinity));
            Assert.Throws<ArgumentException>(() => Validation.ValidateFloat(float.NaN));
            Assert.Throws<ArgumentException>(() => Validation.ValidateFloat(float.PositiveInfinity));

            // string and bool accept only their own CLR type.
            Validation.ValidateString("x");
            Validation.ValidateBool(true);
            Assert.Throws<ArgumentException>(() => Validation.ValidateString(5));
            Assert.Throws<ArgumentException>(() => Validation.ValidateBool(1));
        }
    }
}

