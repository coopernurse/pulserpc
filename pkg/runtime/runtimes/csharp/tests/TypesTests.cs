using System.Collections.Generic;
using Xunit;
using PulseRPC;

namespace PulseRPC.Tests
{
    public class TypesTests
    {
        [Fact]
        public void FindStruct_ReturnsStructDefinition()
        {
            var allStructs = new Dictionary<string, Dictionary<string, object>>
            {
                { "User", new Dictionary<string, object> { { "fields", new List<object>() } } },
                { "Book", new Dictionary<string, object> { { "fields", new List<object>() } } }
            };
            var result = Types.FindStruct("User", allStructs);
            Assert.NotNull(result);
            Assert.True(result!.ContainsKey("fields"));

            result = Types.FindStruct("Book", allStructs);
            Assert.NotNull(result);
            Assert.True(result!.ContainsKey("fields"));

            result = Types.FindStruct("NotFound", allStructs);
            Assert.Null(result);
        }

        [Fact]
        public void FindEnum_ReturnsEnumDefinition()
        {
            var allEnums = new Dictionary<string, Dictionary<string, object>>
            {
                { "Platform", new Dictionary<string, object> { { "values", new List<object>() } } }
            };
            var result = Types.FindEnum("Platform", allEnums);
            Assert.NotNull(result);
            Assert.True(result!.ContainsKey("values"));

            result = Types.FindEnum("NotFound", allEnums);
            Assert.Null(result);
        }

        [Fact]
        public void GetStructFields_SimpleStruct()
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
                                    { "type", new Dictionary<string, object> { { "builtIn", "string" } } }
                                },
                                new Dictionary<string, object>
                                {
                                    { "name", "name" },
                                    { "type", new Dictionary<string, object> { { "builtIn", "string" } } }
                                }
                            }
                        }
                    }
                }
            };
            var fields = Types.GetStructFields("User", allStructs);
            Assert.Equal(2, fields.Count);
            Assert.Equal("id", fields[0]["name"].ToString());
            Assert.Equal("name", fields[1]["name"].ToString());
        }

        [Fact]
        public void GetStructFields_WithExtends()
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
                                    { "type", new Dictionary<string, object> { { "builtIn", "string" } } }
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
                                    { "type", new Dictionary<string, object> { { "builtIn", "string" } } }
                                }
                            }
                        }
                    }
                }
            };
            var fields = Types.GetStructFields("User", allStructs);
            Assert.Equal(2, fields.Count);
            Assert.Equal("id", fields[0]["name"].ToString()); // Parent field first
            Assert.Equal("name", fields[1]["name"].ToString()); // Child field second
        }

        [Fact]
        public void GetStructFields_OverrideParent()
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
                                    { "type", new Dictionary<string, object> { { "builtIn", "string" } } }
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
                                    { "name", "id" },
                                    { "type", new Dictionary<string, object> { { "builtIn", "int" } } } // Override parent
                                },
                                new Dictionary<string, object>
                                {
                                    { "name", "name" },
                                    { "type", new Dictionary<string, object> { { "builtIn", "string" } } }
                                }
                            }
                        }
                    }
                }
            };
            var fields = Types.GetStructFields("User", allStructs);
            Assert.Equal(2, fields.Count);
            // Child field should override parent
            var idFieldType = fields[0]["type"] as Dictionary<string, object>;
            Assert.NotNull(idFieldType);
            Assert.Equal("int", idFieldType!["builtIn"].ToString());
            Assert.Equal("name", fields[1]["name"].ToString());
        }

        // csharp-07: qualified struct names must resolve like qualified enums.
        [Fact]
        public void FindStruct_QualifiedName_FallsBackToBaseName()
        {
            var allStructs = new Dictionary<string, Dictionary<string, object>>
            {
                { "User", new Dictionary<string, object> { { "fields", new List<object>() } } }
            };

            Assert.Same(allStructs["User"], Types.FindStruct("inc.User", allStructs));
        }

        [Fact]
        public void FindStruct_QualifiedName_MatchesRegisteredQualifiedName()
        {
            var allStructs = new Dictionary<string, Dictionary<string, object>>
            {
                { "inc.User", new Dictionary<string, object> { { "fields", new List<object>() } } }
            };

            Assert.Same(allStructs["inc.User"], Types.FindStruct("inc.User", allStructs));
        }

        [Fact]
        public void FindStruct_AmbiguousBaseName_ResolvesDeterministically()
        {
            var allStructs = new Dictionary<string, Dictionary<string, object>>
            {
                { "z.User", new Dictionary<string, object> { { "fields", new List<object>() } } },
                { "a.User", new Dictionary<string, object> { { "fields", new List<object>() } } }
            };

            // Two namespaces define "User"; resolution is stable (first ordinal key).
            Assert.Same(allStructs["a.User"], Types.FindStruct("other.User", allStructs));
            Assert.Same(allStructs["a.User"], Types.FindStruct("other.User", allStructs));
        }

        [Fact]
        public void ValidateType_QualifiedStructName_Resolves()
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
                                    { "name", "city" },
                                    { "type", new Dictionary<string, object> { { "builtIn", "string" } } },
                                    { "optional", false }
                                }
                            }
                        }
                    }
                }
            };
            var allEnums = new Dictionary<string, Dictionary<string, object>>();
            var typeDef = new Dictionary<string, object> { { "userDefined", "inc.User" } };

            Validation.ValidateType(
                new Dictionary<string, object?> { { "city", "Paris" } }, typeDef, allStructs, allEnums);

            Assert.Throws<ArgumentException>(() => Validation.ValidateType(
                new Dictionary<string, object?>(), typeDef, allStructs, allEnums));
        }
    }
}

