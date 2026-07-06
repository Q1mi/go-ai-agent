package rag

import "sort"

// RRF 使用倒数排名融合多路检索结果。
// RRF（Reciprocal Rank Fusion，倒数排名融合）是当前 RAG 多路召回场景的**工业标准无监督融合算法**，
// 它完全基于排名位置计算综合得分，绕过了不同检索通路分数量纲不一致的核心痛点，
// 实现简单、零调参、鲁棒性极强，是混合检索（向量 + 关键词）结果融合的首选方案。
// 向量检索的余弦相似度是 0~1 的连续值，
// BM25 分数是 0~ 几十的离散值，
// 标签匹配是 0/1 二值，
// 三者量纲完全不同，手动加权调参极其困难。
// RRF 完全抛弃绝对分数，只看相对排名，无需任何归一化操作即可直接融合。
func RRF(rankings [][]string, k int) []string {
	if k <= 0 {
		k = 60
	}
	score := make(map[string]float64)
	for _, ranking := range rankings {
		for rank, id := range ranking {
			if id == "" {
				continue
			}
			score[id] += 1.0 / float64(k+rank+1)
		}
	}
	ids := make([]string, 0, len(score))
	for id := range score {
		ids = append(ids, id)
	}
	sort.SliceStable(ids, func(i, j int) bool {
		if score[ids[i]] == score[ids[j]] {
			return ids[i] < ids[j]
		}
		return score[ids[i]] > score[ids[j]]
	})
	return ids
}
