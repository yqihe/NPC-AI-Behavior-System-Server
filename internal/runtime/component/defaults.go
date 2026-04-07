package component

// DefaultRegistry 返回注册了全部 10 个组件工厂的注册表
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register("identity", IdentityFactory)
	r.Register("position", PositionFactory)
	r.Register("behavior", BehaviorFactory)
	r.Register("perception", PerceptionFactory)
	r.Register("movement", MovementFactory)
	r.Register("personality", PersonalityFactory)
	r.Register("needs", NeedsFactory)
	r.Register("emotion", EmotionFactory)
	r.Register("memory", MemoryFactory)
	r.Register("social", SocialFactory)
	return r
}
